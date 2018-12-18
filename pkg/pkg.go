package pkg

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/srelab/url-shortener/pkg/handlers"
	"github.com/srelab/url-shortener/pkg/logger"
	"github.com/srelab/url-shortener/pkg/stores"
)

func Start() (func(), error) {
	store, err := stores.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not create store")
	}

	handler, err := handlers.New(*store)
	if err != nil {
		return nil, errors.Wrap(err, "could not create handlers")
	}

	go func() {
		if err := handler.Listen(); err != nil {
			logger.Fatalf("could not listen to http handlers: %v", err)
		}
	}()

	return func() {
		if err = handler.CloseStore(); err != nil {
			fmt.Println(fmt.Sprintf("failed to stop the handlers: %v", err))
		}
	}, nil
}
