package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
	return &Runtime{
		Options: opts,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}
}

func (rt *Runtime) Client() *adminclient.Client {
	client := adminclient.New(rt.Options.BaseURL, rt.HTTPClient)
	client.SetToken(strings.TrimSpace(rt.Options.Token))
	return client
}

func (rt *Runtime) Context() context.Context {
	return context.Background()
}

func (rt *Runtime) AuthenticatedClient(ctx context.Context) (*adminclient.Client, error) {
	client := rt.Client()
	if strings.TrimSpace(client.Token()) != "" {
		return client, nil
	}

	username := strings.TrimSpace(rt.Options.Username)
	password := strings.TrimSpace(rt.Options.Password)
	if username == "" || password == "" {
		return nil, errors.New("authentication required: provide --token or both --username and --password")
	}

	resp, err := client.Login(ctx, username, password)
	if err != nil {
		return nil, err
	}

	rt.Options.Token = resp.Token
	client.SetToken(resp.Token)
	return client, nil
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

func (rt *Runtime) Printf(format string, args ...any) error {
	target := rt.Stdout
	if target == nil {
		target = io.Discard
	}
	_, err := fmt.Fprintf(target, format, args...)
	return err
}
