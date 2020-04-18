package main

import (
	"context"
	"log"
	"time"

	launchctl "github.com/jmacd/launchmidi/launchctl/xl"
)

func main() {
	ctl, err := launchctl.Open()
	if err != nil {
		log.Fatalf("error while openning connection to launchctl: %v", err)
	}
	defer ctl.Close()

	ctx := context.Background()
	ctx, _ = context.WithTimeout(ctx, time.Second*5)

	if err := ctl.Run(ctx); err != nil {
		log.Fatal("error running launchctl: ", err)
	}
}
