// Command credits adjusts a user's prepaid AI budget by hand, until a
// payment provider calls credits.Ledger.RecordPurchase directly.
//
//	go run ./cmd/credits balance  -user <uuid>
//	go run ./cmd/credits grant    -user <uuid> -usd 1.00 -note "beta tester"
//	go run ./cmd/credits purchase -user <uuid> -price 5 -pack starter -ref <payment id>
//
// A purchase adds 80% of the price as budget; the same -ref twice credits
// once. Reads DATABASE_URL and STARTER_CREDITS_USD like the API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/franco-roura/scrapbook-01/server/internal/auth"
	"github.com/franco-roura/scrapbook-01/server/internal/config"
	"github.com/franco-roura/scrapbook-01/server/internal/credits"
	"github.com/franco-roura/scrapbook-01/server/internal/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "credits:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: credits balance|grant|purchase -user <user_... or uuid> [...]")
	}
	cmd := args[0]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	userFlag := fs.String("user", "", "WorkOS user id (user_...) or our user uuid")
	usd := fs.Float64("usd", 0, "grant: model budget to add, in dollars")
	note := fs.String("note", "", "grant: why")
	price := fs.Float64("price", 0, "purchase: price paid, in dollars")
	pack := fs.String("pack", "", "purchase: pack name")
	ref := fs.String("ref", "", "unique reference (payment id); repeats are ignored")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	user := auth.UserIDFor(*userFlag)
	if user == uuid.Nil {
		return fmt.Errorf("-user is required")
	}

	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	starter, _ := strconv.ParseFloat(os.Getenv("STARTER_CREDITS_USD"), 64)
	if os.Getenv("STARTER_CREDITS_USD") == "" {
		starter = 0.10
	}
	l := &credits.Ledger{Q: db.New(pool), Starter: int64(starter * credits.MicroPerDollar)}

	var refp *string
	if *ref != "" {
		refp = ref
	}
	var bal int64
	applied := true
	switch cmd {
	case "balance":
		bal, err = l.Balance(ctx, user)
	case "grant":
		bal, applied, err = l.Grant(ctx, user, int64(*usd*credits.MicroPerDollar), *note, refp)
	case "purchase":
		bal, applied, err = l.RecordPurchase(ctx, user, *price, *pack, *ref)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		return err
	}
	if !applied {
		fmt.Println("reference already recorded; nothing changed")
	}
	fmt.Printf("balance: $%.6f of model budget\n", credits.Dollars(bal))
	return nil
}
