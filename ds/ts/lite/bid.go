package lite

import (
	"database/sql"
	"errors"

	ts "github.com/solpipe/solpipe-tool/ds/ts"
)

func (e1 external) BidAppend(bp ts.BidPoint) error {
	return e1.tx(func(tx *sql.Tx) error {
		return e1.inside_bidappend(tx, bp)
	})
}

const SQL_BID_INSERT_1 string = `
WITH x as (
	SELECT b1."id" FROM "bidders" b1 WHERE b1.bidder_id = $1
	UNION
	INSERT INTO "bidder" (bidder_id) VALUES ($1) RETURNING "id"
),
INSERT INTO "bid_snapshot" (bidder,pipeline,start,slot,deposit)
SELECT x."id", p1.pipeline, p1.start, $3, $4
FROM x CROSS JOIN "payout" p1
WHERE p1.payout_id = $2
`

func (e1 external) inside_bidappend(tx *sql.Tx, bp ts.BidPoint) error {
	insertStmt, err := tx.PrepareContext(e1.ctx, SQL_BID_INSERT_1)
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	var result sql.Result
	for _, bid := range bp.Status.Bid {
		if !bid.IsBlank {
			result, err = insertStmt.ExecContext(e1.ctx,
				bid.User.String(),
				bp.PayoutId.String(),
				bp.Slot,
				bid.Deposit,
			)
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n == 0 {
				return errors.New("failed to insert")
			}
		}
	}

	return nil
}
