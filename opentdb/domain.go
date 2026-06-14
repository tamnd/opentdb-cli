package opentdb

import (
	"context"
	"fmt"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes opentdb as a kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/opentdb-cli/opentdb"
//
// The same Domain also builds the standalone opentdb binary (see cli.NewApp).
func init() { kit.Register(Domain{}) }

// Domain is the opentdb driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "opentdb",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "opentdb",
			Short:  "Open Trivia Database — free trivia questions",
			Long: `opentdb fetches trivia questions and category lists from opentdb.com.
No API key required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/opentdb-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "questions",
		Group:   "read",
		List:    true,
		Summary: "Fetch trivia questions",
	}, questionsOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "categories",
		Group:   "read",
		List:    true,
		Summary: "List all trivia categories",
	}, categoriesOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "count",
		Group:   "read",
		List:    false,
		Summary: "Get question count breakdown for a category",
	}, countOp)
}

// newClient builds the client from host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type questionsInput struct {
	Amount     int     `kit:"flag,inherit" help:"number of questions (1-50)" default:"5"`
	Category   int     `kit:"flag" help:"category ID (0=any)" default:"0"`
	Difficulty string  `kit:"flag" help:"difficulty: easy|medium|hard (empty=any)"`
	Type       string  `kit:"flag" help:"type: multiple|boolean (empty=any)"`
	Client     *Client `kit:"inject"`
}

type categoriesInput struct {
	Client *Client `kit:"inject"`
}

type countInput struct {
	Category int     `kit:"arg" help:"category ID"`
	Client   *Client `kit:"inject"`
}

// --- handlers ---

func questionsOp(ctx context.Context, in questionsInput, emit func(Question) error) error {
	amount := in.Amount
	if amount <= 0 {
		amount = 5
	}
	questions, err := in.Client.Questions(ctx, amount, in.Category, in.Difficulty, in.Type)
	if err != nil {
		return mapErr(err)
	}
	for _, q := range questions {
		if err := emit(q); err != nil {
			return err
		}
	}
	return nil
}

func categoriesOp(ctx context.Context, in categoriesInput, emit func(Category) error) error {
	cats, err := in.Client.Categories(ctx)
	if err != nil {
		return mapErr(err)
	}
	for _, cat := range cats {
		if err := emit(cat); err != nil {
			return err
		}
	}
	return nil
}

func countOp(ctx context.Context, in countInput, emit func(CategoryCount) error) error {
	cc, err := in.Client.Count(ctx, in.Category)
	if err != nil {
		return mapErr(err)
	}
	return emit(cc)
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
// A numeric input maps to type "category"; anything else maps to "query".
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty opentdb reference")
	}
	// Check if it's a numeric category ID.
	var n int
	if _, scanErr := fmt.Sscanf(input, "%d", &n); scanErr == nil {
		return "category", input, nil
	}
	return "query", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "category":
		return fmt.Sprintf("https://opentdb.com/browse.php?category=%s", id), nil
	case "query":
		return "https://opentdb.com/", nil
	default:
		return "", errs.Usage("opentdb has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
