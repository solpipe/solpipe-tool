package lite

import (
	"context"
	"database/sql"

	log "github.com/sirupsen/logrus"
	ts "github.com/solpipe/solpipe-tool/ds/ts"
)

func (e1 external) Initialize(initialState ts.InitialState) error {
	var err error
	list := []string{
		SQL_TIME_CREATE,
		SQL_BIDDER_CREATE_1,
		SQL_PIPELINE_CREATE_1,
		SQL_NETWORK_CREATE_1,
		SQL_PAYOUT_CREATE_1,
		SQL_PAYOUT_BID_CREATE_1,
	}
	for _, sqlStmt := range list {
		_, err = e1.db.Exec(sqlStmt)
		if err != nil {
			log.Info("i - 2")
			return err
		}
	}

	err = e1.insert_time(initialState)
	if err != nil {
		log.Info("i - 3")
		return err
	}

	return nil
}

const SQL_TIME_CREATE string = `
CREATE TABLE IF NOT EXISTS "time"
(
    slot bigint NOT NULL,
    CONSTRAINT time_pkey PRIMARY KEY (slot)
);

CREATE UNIQUE INDEX IF NOT EXISTS time_idx
    ON "time"
    (slot ASC);
`

const SQL_PIPELINE_CREATE_1 string = `
CREATE TABLE IF NOT EXISTS pipeline
(
    id INTEGER PRIMARY KEY,
	pipeline_id character varying(128) NOT NULL,
    CONSTRAINT pipeline_id_uq UNIQUE (pipeline_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS pipeline_id_idx
    ON "pipeline"
    (id ASC);

CREATE UNIQUE INDEX IF NOT EXISTS pipeline_pubkey_idx
    ON "pipeline"
    (pipeline_id ASC);
`

const SQL_BIDDER_CREATE_1 string = `
CREATE TABLE IF NOT EXISTS bidder
(
    id INTEGER PRIMARY KEY,
	bidder_id character varying(128) NOT NULL,
    CONSTRAINT bidder_id_uq UNIQUE (bidder_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS bidder_id_idx
    ON "bidder"
    (id ASC);

CREATE UNIQUE INDEX IF NOT EXISTS bidder_pubkey_idx
    ON "bidder"
    (bidder_id ASC);
`

const SQL_NETWORK_CREATE_1 string = `
CREATE TABLE IF NOT EXISTS "network"
(
    slot INTEGER NOT NULL,
    tps numeric NOT NULL,
    CONSTRAINT network_pkey PRIMARY KEY (slot),
    CONSTRAINT network_slot_fk FOREIGN KEY (slot)
        REFERENCES "time" (slot) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        DEFERRABLE
);

CREATE UNIQUE INDEX IF NOT EXISTS network_slot_idx
    ON "network"
    (slot ASC);
`

const SQL_PAYOUT_CREATE_1 string = `
CREATE TABLE IF NOT EXISTS "payout"
(
	pipeline INTEGER NOT NULL,
    start INTEGER NOT NULL,
	length INTEGER NOT NULL,
    CONSTRAINT payout_pkey PRIMARY KEY (pipeline,start),
	CONSTRAINT payout_pipeline_fk FOREIGN KEY (pipeline)
        REFERENCES "pipeline" ("id") MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        DEFERRABLE,
    CONSTRAINT payout_start_fk FOREIGN KEY (start)
        REFERENCES "time" ("slot") MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        DEFERRABLE
);

CREATE UNIQUE INDEX IF NOT EXISTS payout_pipeline_start_idx
    ON "payout"
    (pipeline ASC, start ASC);
`

const SQL_PAYOUT_BID_CREATE_1 string = `
CREATE TABLE IF NOT EXISTS "bid_snapshot"
(
	slot INTEGER NOT NULL,
	pipeline INTEGER NOT NULL,
	start INTEGER NOT NULL,
    bidder INTEGER NOT NULL,
    CONSTRAINT payout_pkey PRIMARY KEY (slot,pipeline,start),
    CONSTRAINT bid_payout_pipeline_fk FOREIGN KEY (pipeline)
        REFERENCES "pipeline" ("id") MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        DEFERRABLE,
	CONSTRAINT bid_payout_start_fk FOREIGN KEY (start)
        REFERENCES "time" ("slot") MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        DEFERRABLE,
    CONSTRAINT bid_payout_slot_fk FOREIGN KEY (slot)
        REFERENCES "time" ("slot") MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        DEFERRABLE,
	CONSTRAINT bid_bidder_fk FOREIGN KEY (bidder)
        REFERENCES "bidder" ("id") MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        DEFERRABLE
);

CREATE UNIQUE INDEX IF NOT EXISTS bidder_slot_pipeline_start_idx
    ON "bidder"
    (pipeline ASC, start ASC,slot ASC);

CREATE UNIQUE INDEX IF NOT EXISTS bidder_bidder_idx
    ON "bidder"
    (bidder ASC);
`

const SQL_TIME_INSERT string = `
INSERT INTO "time" (slot) VALUES ($1);
`

func (e1 external) insert_time(is ts.InitialState) error {
	ctx := e1.ctx
	tx, err := e1.db.Begin()
	if err != nil {
		return err
	}

	sqlStmt, err := tx.PrepareContext(ctx, SQL_TIME_INSERT)
	if err != nil {
		return err
	}
	for i := is.Start; i < is.Finish; i++ {
		_, err = sqlStmt.ExecContext(ctx, i)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

type sqlGroup struct {
	ctx              context.Context
	db               *sql.DB
	pipeline_select  *sql.Stmt
	stake_select_all *sql.Stmt
}

func sql_init(ctx context.Context, db *sql.DB) (*sqlGroup, error) {
	ans := new(sqlGroup)
	ans.ctx = ctx
	ans.db = db

	err := ans.pipeline()
	if err != nil {
		return nil, err
	}
	err = ans.stake()
	if err != nil {
		return nil, err
	}

	return ans, nil
}
