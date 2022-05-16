package keeper

import (
	"context"
	"strconv"
	"strings"

	"github.com/captoshkala/checkers/x/checkers/rules"
	"github.com/captoshkala/checkers/x/checkers/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) PlayMove(goCtx context.Context, msg *types.MsgPlayMove) (*types.MsgPlayMoveResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: Handling the message
	storedGame, found := k.Keeper.GetStoredGame(ctx, msg.IdValue)
	if !found {
		return nil, sdkerrors.Wrapf(types.ErrGameNotFound, "game not found %s", msg.IdValue)
	}

	if storedGame.Winner != rules.NO_PLAYER.Color {
		return nil, types.ErrGameFinished
	}
	var player rules.Player
	if strings.Compare(storedGame.Red, msg.Creator) == 0 {
		player = rules.RED_PLAYER
	} else if strings.Compare(storedGame.Black, msg.Creator) == 0 {
		player = rules.BLACK_PLAYER
	} else {
		return nil, types.ErrCreatorNotPlayer
	}
	game, err := storedGame.ParseGame()
	if err != nil {
		panic(err.Error())
	}
	if !game.TurnIs(player) {
		return nil, types.ErrNotPlayerTurn
	}

	err = k.Keeper.CollectWager(ctx, &storedGame)
	if err != nil {
		return nil, err
	}
	captured, moveErr := game.Move(
		rules.Pos{
			X: int(msg.FromX),
			Y: int(msg.FromY),
		},
		rules.Pos{
			X: int(msg.ToX),
			Y: int(msg.ToY),
		},
	)
	if moveErr != nil {
		return nil, sdkerrors.Wrapf(moveErr, types.ErrWrongMove.Error())
	}
	storedGame.MoveCount++
	storedGame.Deadline = types.FormatDeadline(types.GetNextDeadline(ctx))
	nextGame, found := k.Keeper.GetNextGame(ctx)
	if !found {
		panic("Next game not found")
	}
	if storedGame.Winner == rules.NO_PLAYER.Color {
		k.Keeper.sendToFifoTail(ctx, &storedGame, &nextGame)
	} else {
		k.Keeper.RemoveFromFifo(ctx, &storedGame, &nextGame)
		k.Keeper.MustPayWinnings(ctx, &storedGame)
	}

	storedGame.Game = game.String()
	storedGame.Turn = game.Turn.Color
	storedGame.Winner = game.Winner().Color
	k.Keeper.SetStoredGame(ctx, storedGame)
	k.Keeper.SetNextGame(ctx, nextGame)
	ctx.GasMeter().ConsumeGas(types.PlayMoveGas, "play a move")
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, "checkers"),
			sdk.NewAttribute(sdk.AttributeKeyAction, types.PlayMoveEventKey),
			sdk.NewAttribute(types.PlayMoveEventCreator, msg.Creator),
			sdk.NewAttribute(types.PlayMoveEventIdValue, msg.IdValue),
			sdk.NewAttribute(types.PlayMoveEventCapturedX, strconv.FormatInt(int64(captured.X), 10)),
			sdk.NewAttribute(types.PlayMoveEventCapturedY, strconv.FormatInt(int64(captured.Y), 10)),
			sdk.NewAttribute(types.PlayMoveEventWinner, game.Winner().Color),
		),
	)
	return &types.MsgPlayMoveResponse{
		IdValue:   msg.IdValue,
		CapturedX: int64(captured.X),
		CapturedY: int64(captured.Y),
		Winner:    game.Winner().Color,
	}, nil
}
