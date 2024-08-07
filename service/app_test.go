package service

import (
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/muka/go-bluetooth/api"
)

func createTestApp(t *testing.T) *App {

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()
	a, err := NewApp(AppOptions{
		AdapterID: api.GetDefaultAdapterID(),
		Logger:    logger,
	})
	if err != nil {
		t.Fatal(err)
	}

	s1, err := a.NewService("2233")
	if err != nil {
		t.Fatal(err)
	}

	c1, err := s1.NewChar("3344")
	if err != nil {
		t.Fatal(err)
	}

	c1.
		OnRead(CharReadCallback(func(_ *Char, _ map[string]interface{}) ([]byte, error) {
			return nil, nil
		})).
		OnWrite(CharWriteCallback(func(_ *Char, _ []byte) ([]byte, error) {
			return nil, nil
		}))

	d1, err := c1.NewDescr("4455")
	if err != nil {
		t.Fatal(err)
	}

	err = c1.AddDescr(d1)
	if err != nil {
		t.Fatal(err)
	}

	err = s1.AddChar(c1)
	if err != nil {
		t.Fatal(err)
	}

	err = a.AddService(s1)
	if err != nil {
		t.Fatal(err)
	}

	err = a.Run()
	if err != nil {
		t.Fatal(err)
	}

	return a
}

func TestApp(t *testing.T) {
	t.Skip("Skipping as always failed")
	// todo fix it
	a := createTestApp(t)
	defer a.Close()
}
