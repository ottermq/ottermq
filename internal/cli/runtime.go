package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	adminclient "github.com/ottermq/ottermq/pkg/adminapi/client"
)

type Runtime struct {
	Options    *RootOptions
	HTTPClient *http.Client
	Stdout     io.Writer
	Stderr     io.Writer
}

func NewRuntime(opts *RootOptions) *Runtime {
	if opts == nil {
		opts = &RootOptions{}
	}
	return &Runtime{Options: opts}
}

func (rt *Runtime) Client() *adminclient.Client {
	client := adminclient.New(rt.Options.BaseURL, rt.HTTPClient)
	client.SetToken(strings.TrimSpace(rt.Options.Token))
	return client
}

func (rt *Runtime) Context() context.Context {
	return context.Background()
}

func (rt *Runtime) PrintJSON(v any) error {
	target := rt.Stdout
	if target == nil {
		target = io.Discard
	}

	enc := json.NewEncoder(target)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (rt *Runtime) Println(msg string) error {
	target := rt.Stdout
	if target == nil {
		target = io.Discard
	}
	_, err := fmt.Fprintln(target, msg)
	return err
}

