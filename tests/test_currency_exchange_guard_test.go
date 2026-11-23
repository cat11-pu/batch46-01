package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-02"
)

func exchangeCfg() taskpkg.Config {
	return taskpkg.Config{
		FeeBasisPoints: 250,
		MaxRateAge:     30 * time.Minute,
		HoldTTL:        time.Hour,
	}
}

func exchangeStart() time.Time { return time.Unix(1730000000, 0) }

// newExchange opens acc-77 with 2000 GLD and 500 GEM and publishes GLD->GEM
// at 2:1.
func newExchange(t *testing.T) (*taskpkg.CurrencyExchange, time.Time) {
	t.Helper()
	now := exchangeStart()
	x := taskpkg.NewCurrencyExchange(exchangeCfg())
	if err := x.OpenAccount("acc-77"); err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	if err := x.Credit("acc-77", "GLD", 2000); err != nil {
		t.Fatalf("Credit GLD: %v", err)
	}
	if err := x.Credit("acc-77", "GEM", 500); err != nil {
		t.Fatalf("Credit GEM: %v", err)
	}
	if err := x.SetRate("GLD", "GEM", 2, 1, now); err != nil {
		t.Fatalf("SetRate GLD->GEM: %v", err)
	}
	return x, now
}

// Balances must be readable, credits validated and unknown accounts told apart.
func TestOpenAccountCreditAndBalance(t *testing.T) {
	x, _ := newExchange(t)
	b, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if b.Available != 2000 || b.Frozen != 0 || b.Total != 2000 {
		t.Fatalf("GLD balance = %+v, want 2000/0/2000", b)
	}
	if _, err := x.Balance("acc-ghost", "GLD"); !errors.Is(err, taskpkg.ErrUnknownAccount) {
		t.Fatalf("Balance of unknown account = %v, want ErrUnknownAccount", err)
	}
	if err := x.Credit("acc-77", "GLD", 0); !errors.Is(err, taskpkg.ErrInvalidAmount) {
		t.Fatalf("Credit(0) = %v, want ErrInvalidAmount", err)
	}
	if err := x.Credit("acc-77", "GLD", -5); !errors.Is(err, taskpkg.ErrInvalidAmount) {
		t.Fatalf("Credit(-5) = %v, want ErrInvalidAmount", err)
	}
	if err := x.Credit("acc-ghost", "GLD", 10); !errors.Is(err, taskpkg.ErrUnknownAccount) {
		t.Fatalf("Credit to unknown account = %v, want ErrUnknownAccount", err)
	}
	after, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance after rejected credits: %v", err)
	}
	if after.Available != 2000 {
		t.Fatalf("rejected credits changed GLD to %d, want 2000", after.Available)
	}
}

// Currency codes are exactly three upper-case letters.
func TestInvalidCurrencyCodeRejected(t *testing.T) {
	x, _ := newExchange(t)
	for _, code := range []string{"gl", "GLDX", "G0D", "", "Gld"} {
		if err := x.Credit("acc-77", code, 100); !errors.Is(err, taskpkg.ErrInvalidCurrency) {
			t.Fatalf("Credit with currency %q = %v, want ErrInvalidCurrency", code, err)
		}
	}
	b, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if b.Available != 2000 {
		t.Fatalf("GLD after rejected currency codes = %d, want 2000", b.Available)
	}
}

// Quote applies rate and fee and must not touch the wallet.
func TestQuoteAppliesRateAndFee(t *testing.T) {
	x, now := newExchange(t)
	q, err := x.Quote("GLD", "GEM", 1000, now)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if q.Gross != 2000 {
		t.Fatalf("gross = %d, want 2000", q.Gross)
	}
	if q.Fee != 50 {
		t.Fatalf("fee = %d, want 50 (2.5%% of 2000)", q.Fee)
	}
	if q.Net != 1950 {
		t.Fatalf("net = %d, want 1950", q.Net)
	}
	b, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if b.Available != 2000 {
		t.Fatalf("Quote changed the GLD balance to %d, want 2000", b.Available)
	}
}

// A rate is directed: the reverse direction and the same currency are refused.
func TestQuoteRejectsMissingAndReverseRate(t *testing.T) {
	x, now := newExchange(t)
	if _, err := x.Quote("GEM", "GLD", 100, now); !errors.Is(err, taskpkg.ErrRateNotFound) {
		t.Fatalf("reverse quote = %v, want ErrRateNotFound", err)
	}
	if _, err := x.Quote("GLD", "GLD", 100, now); !errors.Is(err, taskpkg.ErrSameCurrency) {
		t.Fatalf("same-currency quote = %v, want ErrSameCurrency", err)
	}
	if _, err := x.Quote("GLD", "GEM", 0, now); !errors.Is(err, taskpkg.ErrInvalidAmount) {
		t.Fatalf("zero-amount quote = %v, want ErrInvalidAmount", err)
	}
}

// A rate older than MaxRateAge must not be used.
func TestStaleRateRejected(t *testing.T) {
	now := exchangeStart()
	x := taskpkg.NewCurrencyExchange(exchangeCfg())
	if err := x.OpenAccount("acc-77"); err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	if err := x.Credit("acc-77", "GLD", 1000); err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if err := x.SetRate("GLD", "GEM", 2, 1, now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("SetRate: %v", err)
	}
	if _, err := x.Quote("GLD", "GEM", 100, now); !errors.Is(err, taskpkg.ErrStaleRate) {
		t.Fatalf("quote with a two-hour-old rate = %v, want ErrStaleRate", err)
	}
	_, err := x.Exchange(taskpkg.ExchangeRequest{RequestID: "ex-stale", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 100}, now)
	if !errors.Is(err, taskpkg.ErrStaleRate) {
		t.Fatalf("exchange with a stale rate = %v, want ErrStaleRate", err)
	}
	b, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if b.Available != 1000 {
		t.Fatalf("stale exchange changed GLD to %d, want 1000", b.Available)
	}
}

// A conversion debits the source, credits the net amount and reports the fee.
func TestExchangeMovesFundsAndReportsFee(t *testing.T) {
	x, now := newExchange(t)
	res, err := x.Exchange(taskpkg.ExchangeRequest{RequestID: "ex-1", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 400}, now)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if res.Spent != 400 || res.Gross != 800 || res.Fee != 20 || res.Net != 780 {
		t.Fatalf("result = %+v, want spent 400 gross 800 fee 20 net 780", res)
	}
	if res.AccountID != "acc-77" || res.From != "GLD" || res.To != "GEM" {
		t.Fatalf("result identifiers = %+v", res)
	}
	gld, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if gld.Available != 1600 || gld.Total != 1600 {
		t.Fatalf("GLD after exchange = %+v, want 1600/0/1600", gld)
	}
	gem, err := x.Balance("acc-77", "GEM")
	if err != nil {
		t.Fatalf("Balance GEM: %v", err)
	}
	if gem.Available != 1280 {
		t.Fatalf("GEM after exchange = %d, want 1280", gem.Available)
	}
}

// Spending more than the available balance must change nothing.
func TestExchangeInsufficientFunds(t *testing.T) {
	x, now := newExchange(t)
	_, err := x.Exchange(taskpkg.ExchangeRequest{RequestID: "ex-big", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 5000}, now)
	if !errors.Is(err, taskpkg.ErrInsufficientFunds) {
		t.Fatalf("oversized exchange = %v, want ErrInsufficientFunds", err)
	}
	gld, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if gld.Available != 2000 {
		t.Fatalf("GLD after a rejected exchange = %d, want 2000", gld.Available)
	}
	gem, err := x.Balance("acc-77", "GEM")
	if err != nil {
		t.Fatalf("Balance GEM: %v", err)
	}
	if gem.Available != 500 {
		t.Fatalf("GEM after a rejected exchange = %d, want 500", gem.Available)
	}
}

// Replaying a request id returns the stored result; reusing it with another
// payload is a conflict.
func TestExchangeIdempotentReplayAndConflict(t *testing.T) {
	x, now := newExchange(t)
	req := taskpkg.ExchangeRequest{RequestID: "ex-9", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 1000}
	first, err := x.Exchange(req, now)
	if err != nil {
		t.Fatalf("first Exchange: %v", err)
	}
	replay, err := x.Exchange(req, now)
	if err != nil {
		t.Fatalf("replayed Exchange: %v", err)
	}
	if replay != first {
		t.Fatalf("replay = %+v, want the stored result %+v", replay, first)
	}
	gld, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if gld.Available != 1000 {
		t.Fatalf("GLD after replay = %d, want 1000 (debited once)", gld.Available)
	}
	conflict := req
	conflict.Amount = 500
	if _, err := x.Exchange(conflict, now); !errors.Is(err, taskpkg.ErrIDConflict) {
		t.Fatalf("conflicting payload for ex-9 = %v, want ErrIDConflict", err)
	}
	if gld, err = x.Balance("acc-77", "GLD"); err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if gld.Available != 1000 {
		t.Fatalf("GLD after the conflict = %d, want 1000", gld.Available)
	}
}

// Exchange and Freeze share the daily cap, and releasing a hold gives the
// reservation back.
func TestDailyCapSharedByExchangeAndFreeze(t *testing.T) {
	now := exchangeStart()
	cfg := exchangeCfg()
	cfg.DailyCap = map[string]int64{"GLD": 1500}
	x := taskpkg.NewCurrencyExchange(cfg)
	if err := x.OpenAccount("acc-77"); err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	if err := x.Credit("acc-77", "GLD", 2000); err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if err := x.SetRate("GLD", "GEM", 2, 1, now); err != nil {
		t.Fatalf("SetRate: %v", err)
	}
	if _, err := x.Exchange(taskpkg.ExchangeRequest{RequestID: "ex-a", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 1000}, now); err != nil {
		t.Fatalf("Exchange 1000: %v", err)
	}
	if used := x.DailyUsed("acc-77", "GLD", now); used != 1000 {
		t.Fatalf("DailyUsed after exchange = %d, want 1000", used)
	}
	if _, err := x.Freeze(taskpkg.HoldRequest{RequestID: "h-1", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 400}, now); err != nil {
		t.Fatalf("Freeze 400: %v", err)
	}
	if used := x.DailyUsed("acc-77", "GLD", now); used != 1400 {
		t.Fatalf("DailyUsed after freeze = %d, want 1400", used)
	}
	if _, err := x.Exchange(taskpkg.ExchangeRequest{RequestID: "ex-b", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 200}, now); !errors.Is(err, taskpkg.ErrDailyCap) {
		t.Fatalf("exchange beyond the cap = %v, want ErrDailyCap", err)
	}
	if err := x.ReleaseHold("h-1", now); err != nil {
		t.Fatalf("ReleaseHold: %v", err)
	}
	if used := x.DailyUsed("acc-77", "GLD", now); used != 1000 {
		t.Fatalf("DailyUsed after release = %d, want 1000", used)
	}
	if _, err := x.Exchange(taskpkg.ExchangeRequest{RequestID: "ex-c", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 200}, now); err != nil {
		t.Fatalf("exchange after release: %v", err)
	}
	gld, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if gld.Available != 800 {
		t.Fatalf("GLD after two exchanges = %d, want 800", gld.Available)
	}
}

// Freezing moves the source amount from Available to Frozen and keeps Total.
func TestFreezeReducesAvailableNotTotal(t *testing.T) {
	x, now := newExchange(t)
	h, err := x.Freeze(taskpkg.HoldRequest{RequestID: "h-1", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 600}, now)
	if err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if h.Amount != 600 || h.Gross != 1200 || h.Fee != 30 || h.Net != 1170 {
		t.Fatalf("hold = %+v, want amount 600 gross 1200 fee 30 net 1170", h)
	}
	if !h.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("hold expiry = %v, want %v", h.ExpiresAt, now.Add(time.Hour))
	}
	b, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if b.Available != 1400 || b.Frozen != 600 || b.Total != 2000 {
		t.Fatalf("GLD while frozen = %+v, want 1400/600/2000", b)
	}
	gem, err := x.Balance("acc-77", "GEM")
	if err != nil {
		t.Fatalf("Balance GEM: %v", err)
	}
	if gem.Available != 500 {
		t.Fatalf("GEM before settlement = %d, want 500", gem.Available)
	}
}

// Invalid freeze requests must leave the wallet untouched.
func TestFreezeRejectsBadRequests(t *testing.T) {
	x, now := newExchange(t)
	cases := []struct {
		req  taskpkg.HoldRequest
		want error
	}{
		{taskpkg.HoldRequest{RequestID: "h-same", AccountID: "acc-77", From: "GLD", To: "GLD", Amount: 100}, taskpkg.ErrSameCurrency},
		{taskpkg.HoldRequest{RequestID: "h-zero", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 0}, taskpkg.ErrInvalidAmount},
		{taskpkg.HoldRequest{RequestID: "h-big", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 99999}, taskpkg.ErrInsufficientFunds},
		{taskpkg.HoldRequest{RequestID: "h-ghost", AccountID: "acc-ghost", From: "GLD", To: "GEM", Amount: 100}, taskpkg.ErrUnknownAccount},
		{taskpkg.HoldRequest{RequestID: "h-norate", AccountID: "acc-77", From: "GEM", To: "GLD", Amount: 100}, taskpkg.ErrRateNotFound},
	}
	for _, c := range cases {
		if _, err := x.Freeze(c.req, now); !errors.Is(err, c.want) {
			t.Fatalf("Freeze(%s) = %v, want %v", c.req.RequestID, err, c.want)
		}
	}
	b, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if b.Available != 2000 || b.Frozen != 0 {
		t.Fatalf("GLD after rejected freezes = %+v, want 2000/0", b)
	}
}

// A hold settles with the quote captured at freeze time, not the current rate.
func TestSettleHoldUsesFreezeTimeRate(t *testing.T) {
	x, now := newExchange(t)
	if _, err := x.Freeze(taskpkg.HoldRequest{RequestID: "h-1", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 600}, now); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if err := x.SetRate("GLD", "GEM", 3, 1, now.Add(time.Minute)); err != nil {
		t.Fatalf("SetRate: %v", err)
	}
	res, err := x.SettleHold("h-1", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("SettleHold: %v", err)
	}
	if res.Net != 1170 || res.Gross != 1200 || res.Fee != 30 || res.Spent != 600 {
		t.Fatalf("settled = %+v, want the freeze-time quote 600/1200/30/1170", res)
	}
	gem, err := x.Balance("acc-77", "GEM")
	if err != nil {
		t.Fatalf("Balance GEM: %v", err)
	}
	if gem.Available != 1670 {
		t.Fatalf("GEM after settling at the old rate = %d, want 1670", gem.Available)
	}
	gld, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if gld.Available != 1400 || gld.Frozen != 0 || gld.Total != 1400 {
		t.Fatalf("GLD after settlement = %+v, want 1400/0/1400", gld)
	}
	if _, err := x.SettleHold("h-1", now.Add(3*time.Minute)); !errors.Is(err, taskpkg.ErrHoldNotFound) {
		t.Fatalf("second settlement = %v, want ErrHoldNotFound", err)
	}
}

// Expired holds must report ErrHoldExpired, then ExpireHolds releases them.
func TestExpireHoldsReleasesFrozenAmount(t *testing.T) {
	x, now := newExchange(t)
	if _, err := x.Freeze(taskpkg.HoldRequest{RequestID: "h-1", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 600}, now); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	late := now.Add(61 * time.Minute)
	if _, err := x.SettleHold("h-1", late); !errors.Is(err, taskpkg.ErrHoldExpired) {
		t.Fatalf("settlement after the TTL = %v, want ErrHoldExpired", err)
	}
	b, err := x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if b.Frozen != 600 {
		t.Fatalf("expired but unsettled hold froze %d, want 600", b.Frozen)
	}
	if n := x.ExpireHolds(late); n != 1 {
		t.Fatalf("ExpireHolds released %d holds, want 1", n)
	}
	b, err = x.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD after expiry: %v", err)
	}
	if b.Available != 2000 || b.Frozen != 0 {
		t.Fatalf("GLD after expiry = %+v, want 2000/0", b)
	}
	if used := x.DailyUsed("acc-77", "GLD", late); used != 0 {
		t.Fatalf("DailyUsed after expiry = %d, want 0", used)
	}
	if _, err := x.SettleHold("h-1", late); !errors.Is(err, taskpkg.ErrHoldNotFound) {
		t.Fatalf("settlement of a released hold = %v, want ErrHoldNotFound", err)
	}
}

// Ten concurrent conversions of 200 GLD against 1000 GLD must never overdraw.
func TestConcurrentExchangesNeverOverdraw(t *testing.T) {
	now := exchangeStart()
	x := taskpkg.NewCurrencyExchange(exchangeCfg())
	if err := x.OpenAccount("acc-race"); err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	if err := x.Credit("acc-race", "GLD", 1000); err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if err := x.SetRate("GLD", "GEM", 2, 1, now); err != nil {
		t.Fatalf("SetRate: %v", err)
	}
	errs := make([]error, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = x.Exchange(taskpkg.ExchangeRequest{
				RequestID: fmt.Sprintf("ex-race-%d", i),
				AccountID: "acc-race",
				From:      "GLD",
				To:        "GEM",
				Amount:    200,
			}, now)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, taskpkg.ErrInsufficientFunds):
			t.Fatalf("concurrent exchange returned an unexpected error: %v", err)
		}
	}
	if ok != 5 {
		t.Fatalf("%d concurrent conversions succeeded, want exactly 5", ok)
	}
	gld, err := x.Balance("acc-race", "GLD")
	if err != nil {
		t.Fatalf("Balance GLD: %v", err)
	}
	if gld.Available != 0 {
		t.Fatalf("GLD left after the race = %d, want 0", gld.Available)
	}
	gem, err := x.Balance("acc-race", "GEM")
	if err != nil {
		t.Fatalf("Balance GEM: %v", err)
	}
	if gem.Available != 1950 {
		t.Fatalf("GEM after five conversions = %d, want 1950", gem.Available)
	}
}

// A restart must keep balances, frozen holds and settled request ids.
func TestSnapshotRestoresBalancesAndIdempotency(t *testing.T) {
	x, now := newExchange(t)
	req := taskpkg.ExchangeRequest{RequestID: "ex-1", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 400}
	first, err := x.Exchange(req, now)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if _, err := x.Freeze(taskpkg.HoldRequest{RequestID: "h-1", AccountID: "acc-77", From: "GLD", To: "GEM", Amount: 600}, now); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	snap, err := x.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewCurrencyExchangeFromSnapshot(exchangeCfg(), snap)
	if err != nil {
		t.Fatalf("NewCurrencyExchangeFromSnapshot: %v", err)
	}
	gld, err := restored.Balance("acc-77", "GLD")
	if err != nil {
		t.Fatalf("restored Balance GLD: %v", err)
	}
	if gld.Available != 1000 || gld.Frozen != 600 || gld.Total != 1600 {
		t.Fatalf("restored GLD = %+v, want 1000/600/1600", gld)
	}
	gem, err := restored.Balance("acc-77", "GEM")
	if err != nil {
		t.Fatalf("restored Balance GEM: %v", err)
	}
	if gem.Available != 1280 {
		t.Fatalf("restored GEM = %d, want 1280", gem.Available)
	}
	replay, err := restored.Exchange(req, now)
	if err != nil {
		t.Fatalf("replayed exchange after restart: %v", err)
	}
	if replay != first {
		t.Fatalf("replay after restart = %+v, want %+v", replay, first)
	}
	if gem, err = restored.Balance("acc-77", "GEM"); err != nil {
		t.Fatalf("Balance GEM: %v", err)
	}
	if gem.Available != 1280 {
		t.Fatalf("GEM credited twice after restart: %d, want 1280", gem.Available)
	}
	res, err := restored.SettleHold("h-1", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SettleHold after restart: %v", err)
	}
	if res.Net != 1170 {
		t.Fatalf("settled hold after restart net = %d, want 1170", res.Net)
	}
}
