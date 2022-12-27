package scheduler

import (
	"context"

	"github.com/solpipe/solpipe-tool/util"
)

func MergeCtx(a context.Context, b context.Context) context.Context {
	return util.MergeCtx(a, b)
}
