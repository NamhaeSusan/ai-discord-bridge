package main

import (
	"context"
	"flag"
	"log"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	configPath := flag.String("config", "", "path to TOML config file")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := ensureCLIs(cfg.Bots); err != nil {
		log.Fatal(err)
	}

	bots, err := NewBots(cfg.Bots)
	if err != nil {
		log.Fatal("failed to create bots: ", err)
	}

	for _, bot := range bots {
		if err := bot.Open(); err != nil {
			log.Fatal("failed to open bot session: ", err)
		}
		defer bot.Close()
	}

	log.Printf("%d Discord bot(s) running. Press Ctrl+C to stop.", len(bots))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Shutting down...")
}

func ensureCLIs(bots []BotConfig) error {
	seen := make(map[string]struct{}, len(bots))
	for _, bot := range bots {
		if _, ok := seen[bot.Provider]; ok {
			continue
		}
		seen[bot.Provider] = struct{}{}

		if _, err := exec.LookPath(bot.Provider); err != nil {
			return err
		}
	}

	return nil
}
