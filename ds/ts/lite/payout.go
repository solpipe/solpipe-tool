package lite

import (
	"database/sql"

	"github.com/solpipe/solpipe-tool/proto/admin"
	"github.com/solpipe/solpipe-tool/state"
	"github.com/solpipe/solpipe-tool/state/sub"
)

func (e1 external) PayoutAppend(list []sub.PayoutWithData) error {

	return e1.tx(func(tx *sql.Tx) error {
		return e1.inside_payout_append(tx, list)
	})
}

const SQL_PAYOUT_INSERT_1 string = `
INSERT INTO "payout" (pipeline,start,payout_id,length,controller_fee,crank_fee,tick_size)
VALUES ($1,$2,$3,$4,$5,$6,$7);
`

func (e1 external) inside_payout_append(tx *sql.Tx, list []sub.PayoutWithData) error {
	ctx := e1.ctx
	insertStmt, err := tx.PrepareContext(ctx, SQL_PAYOUT_INSERT_1)
	if err != nil {
		return err
	}
	defer insertStmt.Close()
	var controllerFee float64
	var crankerFee float64
	for _, x := range list {
		controllerFee, err = state.RateFromProto(&admin.Rate{
			Numerator:   x.Data.ControllerFee[0],
			Denominator: x.Data.ControllerFee[1],
		}).Float()
		if err != nil {
			return err
		}
		crankerFee, err = state.RateFromProto(&admin.Rate{
			Numerator:   x.Data.CrankFee[0],
			Denominator: x.Data.CrankFee[1],
		}).Float()
		if err != nil {
			return err
		}
		_, err = insertStmt.ExecContext(ctx,
			x.Data.Pipeline.String(),
			x.Data.Period.Start,
			x.Id.String(),
			x.Data.Period.Length,
			controllerFee,
			crankerFee,
			x.Data.TickSize,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
