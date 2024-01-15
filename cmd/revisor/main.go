package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/navigacontentlab/navigadoc/doc"
	"github.com/navigacontentlab/revisor"
	"github.com/navigacontentlab/revisor/constraints"
	"github.com/navigacontentlab/revisor/internal"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "revisor",
		Usage: "verifies content according to specifications",
		Commands: []*cli.Command{
			{
				Name:   "document",
				Action: documentCommand,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "json",
						Usage: "output results as JSON",
					},
					&cli.StringSliceFlag{
						Name:  "spec",
						Usage: "file path or URL to load spec from",
					},
					&cli.BoolFlag{
						Name:  "naviga-spec",
						Usage: "use the embedded naviga spec",
						Value: true,
					},
				},
			},
			{
				Name:   "serve",
				Action: serveCommand,
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "spec",
						Usage:   "file path or URL to load spec from",
						EnvVars: []string{"SPEC"},
					},
					&cli.BoolFlag{
						Name:    "naviga-spec",
						Usage:   "use the embedded naviga spec",
						Value:   true,
						EnvVars: []string{"USE_NAVIGA_SPEC"},
					},
					&cli.StringFlag{
						Name:    "addr",
						Usage:   "the address to expose the validation API on",
						Value:   ":8000",
						EnvVars: []string{"LISTEN_ADDR"},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func loadConstraints(
	navigaSpec bool, sources ...string,
) ([]revisor.ConstraintSet, error) {
	var list []revisor.ConstraintSet

	if navigaSpec {
		list = append(list, constraints.Naviga())
	}

	for _, source := range sources {
		var co revisor.ConstraintSet

		ref, loader, err := constraintLoader(source)
		if err != nil {
			return nil, err
		}

		err = loader(ref, &co)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to load constraints from %q: %w",
				source, err)
		}

		list = append(list, co)
	}

	return list, nil
}

type loaderFunc func(ref string, o interface{}) error

func constraintLoader(source string) (string, loaderFunc, error) {
	protocol, rest, ok := strings.Cut(source, "://")
	if !ok {
		return source, internal.UnmarshalFile, nil
	}

	switch protocol {
	case "http", "https":
		return source, internal.UnmarshalHTTPResource, nil
	case "file":
		return rest, internal.UnmarshalFile, nil
	}

	return "", nil, fmt.Errorf("unknown protocol %q", protocol)
}

func serveCommand(c *cli.Context) error {
	addr := c.String("addr")
	specs := c.StringSlice("spec")
	navigaSpec := c.Bool("naviga-spec")

	co, err := loadConstraints(navigaSpec, specs...)
	if err != nil {
		return err
	}

	validator, err := revisor.NewValidator(co...)
	if err != nil {
		return fmt.Errorf("failed to create validator: %w", err)
	}

	srv := &http.Server{
		Addr:         addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		var d doc.Document

		dec := json.NewDecoder(r.Body)

		err := dec.Decode(&d)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))

			return
		}

		e := validator.ValidateDocument(&d)
		sort.Slice(e, func(i, j int) bool {
			return e[i].String() < e[j].String()
		})

		// Empty slice looks much better than "null".
		if e == nil {
			e = make([]revisor.ValidationResult, 0)
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		err = enc.Encode(e)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))

			return
		}
	})

	err = srv.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to start listening on %q: %w",
			srv.Addr, err)
	}

	return nil
}

func documentCommand(c *cli.Context) error {
	if !c.Args().Present() {
		return errors.New("supply one or more paths to NavigaDoc documents")
	}

	paths := c.Args().Slice()
	jsonOut := c.Bool("json")
	specs := c.StringSlice("spec")
	navigaSpec := c.Bool("naviga-spec")

	co, err := loadConstraints(navigaSpec, specs...)
	if err != nil {
		return err
	}

	validator, err := revisor.NewValidator(co...)
	if err != nil {
		return fmt.Errorf("failed to create validator: %w", err)
	}

	var hasErrors bool

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	enc.SetIndent("", "  ")

	for _, path := range paths {
		var d doc.Document

		err := internal.UnmarshalFile(path, &d)
		if err != nil {
			return fmt.Errorf("failed to load document from %s: %w",
				path, err)
		}

		e := validator.ValidateDocument(&d)

		switch {
		case jsonOut:
			sort.Slice(e, func(i, j int) bool {
				return e[i].String() < e[j].String()
			})

			_ = enc.Encode(e)

		default:
			if len(e) > 0 {
				fmt.Fprintln(os.Stdout, d.UUID, d.Type, "==>")
			}

			for _, problem := range e {
				fmt.Fprintln(os.Stdout, problem.String())
			}
		}

		hasErrors = hasErrors || len(e) > 0
	}

	if hasErrors {
		return errors.New("documents had validation errors")
	}

	return nil
}
