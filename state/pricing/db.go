package pricing

import "github.com/solpipe/solpipe-tool/util"

type Handle interface {
	util.Base
	Initialize() error
}
