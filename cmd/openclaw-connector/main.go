// OpenClaw Connector：主动连接商家云，接收 task.dispatch，调用本地 OpenClaw（或 echo），回传 task.result/task.failed
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"openclaw-connector/internal/cloud"
	"openclaw-connector/internal/config"
	"openclaw-connector/internal/driver"
	"openclaw-connector/internal/platform"
	"openclaw-connector/internal/ui"
)

func main() {
	cfgPathFlag := flag.String("config", "", "path to config.yaml (default: beside this executable; go run uses ./config.yaml)")
	headless := flag.Bool("headless", false, "no GUI: no tray/subtitle window, WebSocket client only (for SSH servers)")
	pairCode := flag.String("pair", "", "one-shot: 6-digit code from Miaoban OpenClaw screen (implies -headless)")
	flag.Parse()

	if strings.TrimSpace(*pairCode) != "" {
		*headless = true
	}

	cfgPath := *cfgPathFlag
	if cfgPath == "" {
		p, err := config.DefaultConfigPath()
		if err != nil {
			log.Fatalf("config: %v", err)
		}
		cfgPath = p
	}

	if !*headless {
		var err error
		cfgPath, err = config.EnsureFirstRunConfig(cfgPath)
		if err != nil {
			log.Fatalf("config: %v", err)
		}
	} else if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if _, err := config.EnsureFirstRunConfig(cfgPath); err != nil {
			log.Fatalf("config: %v", err)
		}
	}

	cfg, err := config.UnmarshalFromFile(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if pc := strings.TrimSpace(*pairCode); pc != "" {
		cfg.PairCode = pc
		cfg.FirstRun = false
		config.ApplyBundledDefaults(cfg)
	}

	resolveCtx := context.Background()
	if err := config.ResolveDeviceMAC(resolveCtx, cfg, config.NewResolveHTTPClient(), cfgPath); err != nil {
		if strings.TrimSpace(cfg.PairCode) != "" && strings.TrimSpace(cfg.DeviceMAC) == "" {
			log.Fatalf("resolve device_mac: %v", err)
		}
	}

	if pc := strings.TrimSpace(*pairCode); pc != "" {
		cfg.FirstRun = false
		if err := config.Save(cfgPath, cfg); err != nil {
			log.Fatalf("save config: %v", err)
		}
	}

	if *headless && cfg.FirstRun {
		log.Println("headless: 跳过首次向导；请在 config.yaml 中设置 channels 并将 first_run 改为 false")
	}

	cfg, err = config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("赛搏小小助手 (openclaw-connector) starting, server=%s mode=%s headless=%v openclaw_ch=%v telnet_ch=%v",
		cfg.ServerURL, cfg.OpenClaw.Mode, *headless, cfg.ChannelOpenClaw(), cfg.ChannelTelnet())

	if runtime.GOOS == "darwin" && cfg.DictationInject() {
		if platform.AccessibilityTrusted() {
			log.Println("accessibility: trusted (dictation inject ready)")
		} else {
			log.Println("accessibility: not trusted — showing system prompt (enable in Privacy → Accessibility)")
			platform.RequestAccessibilityPrompt()
		}
	}

	if *headless {
		var drv driver.Driver
		if cfg.ChannelOpenClaw() {
			drv = driver.NewFromConfig(cfg)
		} else {
			drv = driver.Echo{}
		}
		for {
			if ctx.Err() != nil {
				return
			}
			cli := cloud.New(cfg, drv)
			err := cli.Run(ctx)
			if ctx.Err() != nil {
				return
			}
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("connector: disconnected: %v; reconnecting in 2s", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				continue
			}
			return
		}
	}

	_ = ui.Run(ctx, cfg, cfgPath)
}
