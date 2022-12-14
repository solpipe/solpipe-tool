package sub

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	bin "github.com/gagliardetto/binary"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type SubscriptionProgramGroup struct {
	ControllerC chan<- dssub.ResponseChannel[cba.Controller]
	ValidatorC  chan<- dssub.ResponseChannel[ValidatorGroup]

	PipelineC      chan<- dssub.ResponseChannel[PipelineGroup]
	RefundC        chan<- dssub.ResponseChannel[cba.Refunds]
	BidListC       chan<- dssub.ResponseChannel[cba.BidList]
	BidSummaryC    chan<- dssub.ResponseChannel[BidSummary]
	PeriodRingC    chan<- dssub.ResponseChannel[cba.PeriodRing]
	StakerManagerC chan<- dssub.ResponseChannel[StakeGroup]
	StakerReceiptC chan<- dssub.ResponseChannel[StakerReceiptGroup]
	ReceiptC       chan<- dssub.ResponseChannel[ReceiptGroup]
	PayoutC        chan<- dssub.ResponseChannel[PayoutWithData]
}

type internalSubscriptionProgramGroup struct {
	controller   *dssub.SubHome[cba.Controller]
	validator    *dssub.SubHome[ValidatorGroup]
	pipeline     *dssub.SubHome[PipelineGroup]
	bidList      *dssub.SubHome[cba.BidList]
	refund       *dssub.SubHome[cba.Refunds]
	bidSummary   *dssub.SubHome[BidSummary]
	periodRing   *dssub.SubHome[cba.PeriodRing]
	stakeManager *dssub.SubHome[StakeGroup]
	stakeReceipt *dssub.SubHome[StakerReceiptGroup]
	receipt      *dssub.SubHome[ReceiptGroup]
	payout       *dssub.SubHome[PayoutWithData]
}

type ProgramAllResult struct {
	Controller   *cba.Controller
	Validator    *ll.Generic[*ValidatorGroup]
	Pipeline     *ll.Generic[*PipelineGroup]
	Refund       map[string]*cba.Refunds
	PeriodRing   map[string]*PeriodGroup
	BidList      map[string]*BidGroup // map payout id -> BidList
	Stake        *ll.Generic[*StakeGroup]
	StakeReceipt *ll.Generic[*StakerReceiptGroup] // new from refactored code
	Receipt      *ll.Generic[*ReceiptGroup]
	Payout       *ll.Generic[*PayoutWithData]
}

func createProgramAllResult() *ProgramAllResult {
	ans := new(ProgramAllResult)
	ans.Validator = ll.CreateGeneric[*ValidatorGroup]()
	ans.Pipeline = ll.CreateGeneric[*PipelineGroup]()
	ans.Refund = make(map[string]*cba.Refunds)
	ans.PeriodRing = make(map[string]*PeriodGroup)
	ans.BidList = make(map[string]*BidGroup)
	ans.Stake = ll.CreateGeneric[*StakeGroup]()
	ans.StakeReceipt = ll.CreateGeneric[*StakerReceiptGroup]()
	ans.Receipt = ll.CreateGeneric[*ReceiptGroup]()
	ans.Payout = ll.CreateGeneric[*PayoutWithData]()
	return ans
}

func FetchProgramAll(ctx context.Context, rpcClient *sgorpc.Client, version vrs.CbaVersion) (*ProgramAllResult, error) {
	controllerId, controllerBump, err := vrs.ControllerId(version)
	if err != nil {
		return nil, err
	}
	r, err := rpcClient.GetProgramAccountsWithOpts(ctx, cba.ProgramID, &sgorpc.GetProgramAccountsOpts{
		Commitment: sgorpc.CommitmentFinalized,
		Encoding:   solana.EncodingBase64,
	})
	if err != nil {
		return nil, err
	}
	ans := createProgramAllResult()
	for i := 0; i < len(r); i++ {
		if 0 < r[i].Account.Lamports {
			data := r[i].Account.Data.GetBinary()
			if len(data) < 8 {
				return nil, errors.New("account is the wrong size")
			}
			del := [8]byte{}
			for i := 0; i < 8; i++ {
				del[i] = data[i]
			}
			if 8 < len(data) {
				c := bin.NewBorshDecoder(data)
				switch del {
				case cba.ControllerDiscriminator:
					x := new(cba.Controller)
					err = c.Decode(x)
					if err == nil {
						if x.ControllerBump == controllerBump && controllerId.Equals(r[i].Pubkey) {
							ans.Controller = x
						}
					}
				case cba.ValidatorManagerDiscriminator:
					x := new(cba.ValidatorManager)
					err = c.Decode(x)
					if err == nil {
						if x.Controller.Equals(controllerId) {
							ans.Validator.Append(&ValidatorGroup{
								Id:     r[i].Pubkey,
								Data:   *x,
								IsOpen: true,
							})
						}
					}
				case cba.PipelineDiscriminator:
					x := new(cba.Pipeline)
					err = c.Decode(x)
					if err == nil {
						if x.Controller.Equals(controllerId) {
							ans.Pipeline.Append(&PipelineGroup{
								Id:     r[i].Pubkey,
								Data:   *x,
								IsOpen: true,
							})
						}
					}

				case cba.PeriodRingDiscriminator:
					x := new(cba.PeriodRing)
					err = c.Decode(x)
					if err == nil {
						ans.PeriodRing[x.Pipeline.String()] = &PeriodGroup{
							Id:     r[i].Pubkey,
							Data:   *x,
							IsOpen: true,
						}
					}
				case cba.RefundsDiscriminator:
					x := new(cba.Refunds)
					err = c.Decode(x)
					if err == nil {
						ans.Refund[x.Pipeline.String()] = x
					}
				case cba.BidListDiscriminator:
					x := new(cba.BidList)
					err = c.Decode(x)
					if err == nil {
						ans.BidList[x.Payout.String()] = &BidGroup{
							Id:     r[i].Pubkey,
							Data:   *x,
							IsOpen: true,
						}
					}
				case cba.StakerManagerDiscriminator:
					x := new(cba.StakerManager)
					err = c.Decode(x)
					if err == nil {
						ans.Stake.Append(&StakeGroup{
							Id:     r[i].Pubkey,
							Data:   *x,
							IsOpen: true,
						})
					}
				case cba.StakerReceiptDiscriminator:
					x := new(cba.StakerReceipt)
					err = c.Decode(x)
					if err == nil {
						ans.StakeReceipt.Append(&StakerReceiptGroup{
							Id:     r[i].Pubkey,
							Data:   *x,
							IsOpen: true,
						})
					}
				case cba.ReceiptDiscriminator:
					x := new(cba.Receipt)
					err = c.Decode(x)
					if err == nil {
						log.Debugf("1____+++++++receipt=%+v", x)
						ans.Receipt.Append(&ReceiptGroup{
							Id:     r[i].Pubkey,
							Data:   *x,
							IsOpen: true,
						})
					}
				case cba.PayoutDiscriminator:
					x := new(cba.Payout)
					err = c.Decode(x)
					if err == nil {
						ans.Payout.Append(&PayoutWithData{
							Id:     r[i].Pubkey,
							Data:   *x,
							IsOpen: true,
						})
					}
				case cba.RefundsDiscriminator:
					x := new(cba.Refunds)
					err = c.Decode(x)
					if err == nil {
						ans.Refund[x.Pipeline.String()] = x
					}
				default:
					log.Debug("no discriminator matches")
				}
				if err != nil {
					return nil, err
				}
			}
		}

	}
	if ans.Controller == nil {
		return nil, errors.New("no controller")
	}

	if err != nil {
		return nil, err
	}
	return ans, nil
}

/*
updateBidC        chan<- util.ResponseChannel[util.BidGroup]
updatePeriodC     chan<- util.ResponseChannel[cba.PeriodRing]
updateBidSummaryC chan<- util.ResponseChannel[util.BidSummary]
*/
func SubscribeProgramAll(
	ctx context.Context,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	errorC chan<- error,
) (*SubscriptionProgramGroup, error) {

	sub, err := wsClient.ProgramSubscribe(cba.ProgramID, sgorpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}

	ans := new(SubscriptionProgramGroup)
	in := new(internalSubscriptionProgramGroup)
	in.controller = dssub.CreateSubHome[cba.Controller]()
	ans.ControllerC = in.controller.ReqC

	in.validator = dssub.CreateSubHome[ValidatorGroup]()
	ans.ValidatorC = in.validator.ReqC
	in.pipeline = dssub.CreateSubHome[PipelineGroup]()
	ans.PipelineC = in.pipeline.ReqC
	in.bidList = dssub.CreateSubHome[cba.BidList]()
	ans.BidListC = in.bidList.ReqC
	in.bidSummary = dssub.CreateSubHome[BidSummary]()
	ans.BidSummaryC = in.bidSummary.ReqC
	in.periodRing = dssub.CreateSubHome[cba.PeriodRing]()
	ans.PeriodRingC = in.periodRing.ReqC
	in.stakeManager = dssub.CreateSubHome[StakeGroup]()
	ans.StakerManagerC = in.stakeManager.ReqC
	in.stakeReceipt = dssub.CreateSubHome[StakerReceiptGroup]()
	ans.StakerReceiptC = in.stakeReceipt.ReqC
	in.receipt = dssub.CreateSubHome[ReceiptGroup]()
	ans.ReceiptC = in.receipt.ReqC
	in.payout = dssub.CreateSubHome[PayoutWithData]()
	ans.PayoutC = in.payout.ReqC
	in.refund = dssub.CreateSubHome[cba.Refunds]()
	ans.RefundC = in.refund.ReqC

	go loopSubscribePipeline(ctx, sub, in, errorC)

	return ans, nil
}

type DATA_TYPE int

const (
	TYPE_CONTROLLER     DATA_TYPE = 0
	TYPE_VALIDATOR      DATA_TYPE = 1
	TYPE_PIPELINE       DATA_TYPE = 2
	TYPE_BIDS           DATA_TYPE = 3
	TYPE_PERIODS        DATA_TYPE = 4
	TYPE_STAKER         DATA_TYPE = 5
	TYPE_PAYOUT         DATA_TYPE = 6
	TYPE_RECEIPT        DATA_TYPE = 7
	TYPE_STAKER_RECEIPT DATA_TYPE = 8
	TYPE_REFUND         DATA_TYPE = 9
)

func loopSubscribePipeline(
	ctx context.Context,
	sub *sgows.ProgramSubscription,
	in *internalSubscriptionProgramGroup,
	errorC chan<- error,
) {

	doneC := ctx.Done()
	streamC := sub.RecvStream()
	streamErrorC := sub.CloseSignal()

	var err error

	D_controller := binary.BigEndian.Uint64(cba.ControllerDiscriminator[:])
	D_validator := binary.BigEndian.Uint64(cba.ValidatorManagerDiscriminator[:])
	D_pipeline := binary.BigEndian.Uint64(cba.PipelineDiscriminator[:])
	D_bidlist := binary.BigEndian.Uint64(cba.BidListDiscriminator[:])
	D_periodring := binary.BigEndian.Uint64(cba.PeriodRingDiscriminator[:])
	D_stake := binary.BigEndian.Uint64(cba.StakerManagerDiscriminator[:])
	D_stakeReceipt := binary.BigEndian.Uint64(cba.StakerReceiptDiscriminator[:])
	D_receipt := binary.BigEndian.Uint64(cba.ReceiptDiscriminator[:])
	D_payout := binary.BigEndian.Uint64(cba.PayoutDiscriminator[:])
	D_refund := binary.BigEndian.Uint64(cba.RefundsDiscriminator[:])

	trackingAccounts := make(map[string]DATA_TYPE)

out:
	for {
		select {

		case <-doneC:
			break out
		case id := <-in.controller.DeleteC: // controller
			in.controller.Delete(id)
		case r := <-in.controller.ReqC:
			in.controller.Receive(r)
		case id := <-in.validator.DeleteC: // validator
			in.validator.Delete(id)
		case r := <-in.validator.ReqC:
			in.validator.Receive(r)
		case id := <-in.pipeline.DeleteC: // pipeline
			in.pipeline.Delete(id)
		case r := <-in.pipeline.ReqC:
			in.pipeline.Receive(r)
		case id := <-in.periodRing.DeleteC: // period ring
			in.periodRing.Delete(id)
		case r := <-in.periodRing.ReqC:
			in.periodRing.Receive(r)
		case id := <-in.bidList.DeleteC: // bid list
			in.bidList.Delete(id)
		case r := <-in.bidList.ReqC:
			in.bidList.Receive(r)
		case id := <-in.bidSummary.DeleteC: // bid summary
			in.bidSummary.Delete(id)
		case r := <-in.bidSummary.ReqC:
			in.bidSummary.Receive(r)
		case id := <-in.stakeManager.DeleteC: // stake
			in.stakeManager.Delete(id)
		case r := <-in.stakeManager.ReqC:
			in.stakeManager.Receive(r)
		case id := <-in.stakeReceipt.DeleteC: // stake
			in.stakeReceipt.Delete(id)
		case r := <-in.stakeReceipt.ReqC:
			in.stakeReceipt.Receive(r)
		case id := <-in.receipt.DeleteC: // receipt
			in.receipt.Delete(id)
		case r := <-in.receipt.ReqC:
			in.receipt.Receive(r)
		case id := <-in.payout.DeleteC: // payout
			in.payout.Delete(id)
		case r := <-in.payout.ReqC:
			in.payout.Receive(r)
		case id := <-in.refund.DeleteC:
			in.refund.Delete(id)
		case r := <-in.refund.ReqC:
			in.refund.Receive(r)
		case err = <-streamErrorC: // error
			break out
		case d := <-streamC:
			x, ok := d.(*sgows.ProgramResult)
			if !ok {
				err = errors.New("bad program result")
				break out
			}

			data := x.Value.Account.Data.GetBinary()

			if 8 <= len(data) && 0 < x.Value.Account.Lamports {

				switch binary.BigEndian.Uint64(data[0:8]) {
				case D_controller:
					y := new(cba.Controller)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.controller.Broadcast(*y)
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_CONTROLLER
				case D_validator:
					y := new(cba.ValidatorManager)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					log.Debugf("-----+++validator=%+v", y)
					in.validator.Broadcast(ValidatorGroup{
						Id:     x.Value.Pubkey,
						Data:   *y,
						IsOpen: true,
					})
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_VALIDATOR
				case D_pipeline:
					y := new(cba.Pipeline)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.pipeline.Broadcast(PipelineGroup{
						Id:     x.Value.Pubkey,
						Data:   *y,
						IsOpen: true,
					})
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_PIPELINE
				case D_bidlist:
					y := new(cba.BidList)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.bidSummary.Broadcast(BidSummary{
						Payout:        y.Payout,
						TotalDeposits: y.TotalDeposits,
					})
					in.bidList.Broadcast(*y)
				case D_periodring:
					y := new(cba.PeriodRing)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.periodRing.Broadcast(*y)
				case D_stake:
					y := new(cba.StakerManager)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.stakeManager.Broadcast(StakeGroup{
						Id:     x.Value.Pubkey,
						Data:   *y,
						IsOpen: true,
					})
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_STAKER
				case D_stakeReceipt:
					// TODO
					y := new(cba.StakerReceipt)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.stakeReceipt.Broadcast(StakerReceiptGroup{
						Id:     x.Value.Pubkey,
						Data:   *y,
						IsOpen: true,
					})
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_STAKER_RECEIPT
				case D_receipt:
					y := new(cba.Receipt)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.receipt.Broadcast(ReceiptGroup{
						Id:     x.Value.Pubkey,
						Data:   *y,
						IsOpen: true,
					})
					log.Debugf("2____+++++++receipt=%+v", y)
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_RECEIPT
				case D_payout:

					y := new(cba.Payout)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					log.Debugf("update on payout with id=%s; data=%+v", x.Value.Pubkey.String(), y)
					in.payout.Broadcast(PayoutWithData{
						Id:     x.Value.Pubkey,
						Data:   *y,
						IsOpen: true,
					})
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_PAYOUT
				case D_refund:
					log.Debugf("update on refund with id=%s", x.Value.Pubkey.String())
					y := new(cba.Refunds)
					err = bin.UnmarshalBorsh(y, data)
					if err != nil {
						break out
					}
					in.refund.Broadcast(*y)
					trackingAccounts[x.Value.Pubkey.String()] = TYPE_REFUND
				default:
				}
			} else if 8 <= len(data) {
				log.Debugf("account=%s has data, but lamports=0", x.Value.Pubkey.String())
			} else {
				// account being deleted
				log.Debugf("delete with id=%s", x.Value.Pubkey.String())
				id := x.Value.Pubkey
				dataType, present := trackingAccounts[id.String()]
				if present {
					switch dataType {
					case TYPE_CONTROLLER:
						// the controller never dies
					case TYPE_VALIDATOR:
						in.validator.Broadcast(ValidatorGroup{
							Id:     id,
							IsOpen: false,
						})
					case TYPE_PIPELINE:
						in.pipeline.Broadcast(PipelineGroup{
							Id:     id,
							IsOpen: false,
						})
					case TYPE_BIDS:
						// bids are taken care of by the Payout account
					case TYPE_PERIODS:
						// periods are taken care of by the Pipeline account
					case TYPE_REFUND:
						// refunds are taken care of by the Pipeline account
					case TYPE_STAKER:
						in.stakeManager.Broadcast(StakeGroup{
							Id:     id,
							IsOpen: false,
						})
					case TYPE_PAYOUT:
						log.Debug("delete payout")
						in.payout.Broadcast(PayoutWithData{
							Id:     id,
							IsOpen: false,
						})
					case TYPE_RECEIPT:
						in.receipt.Broadcast(ReceiptGroup{
							Id:     id,
							IsOpen: false,
						})
					}
				}
			}

		}
	}

	errorC <- err
}

func SubscribeBidList(wsClient *sgows.Client) (*sgows.ProgramSubscription, error) {
	return wsClient.ProgramSubscribe(cba.ProgramID, sgorpc.CommitmentFinalized)
	//return wsClient.ProgramSubscribeWithOpts(cba.ProgramID, sgorpc.CommitmentConfirmed, sgo.EncodingBase64, []sgorpc.RPCFilter{
	//{
	//	DataSize: util.STRUCT_SIZE_BID_LIST,
	//Memcmp: &sgorpc.RPCFilterMemcmp{
	//	Offset: 0, Bytes: cba.BidListDiscriminator[:],
	//},
	//},
	//})

}

func SubscribePeriodRing(wsClient *sgows.Client) (*sgows.ProgramSubscription, error) {
	//prefix := sgo.Base58(base58.Encode(cba.PeriodRingDiscriminator[:]))
	return wsClient.ProgramSubscribe(cba.ProgramID, sgorpc.CommitmentFinalized)
	//return wsClient.ProgramSubscribeWithOpts(cba.ProgramID, sgorpc.CommitmentConfirmed, sgo.EncodingBase64, []sgorpc.RPCFilter{
	//{
	//DataSize: util.STRUCT_SIZE_PERIOD_RING,
	//Memcmp: &sgorpc.RPCFilterMemcmp{
	//	Offset: 0, Bytes: cba.PeriodRingDiscriminator[:],
	//},
	//},
	//})

}
