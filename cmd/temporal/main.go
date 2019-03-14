package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	ci "github.com/libp2p/go-libp2p-crypto"

	"github.com/RTradeLtd/cmd"
	"github.com/RTradeLtd/config"
	"github.com/RTradeLtd/database"
	pb "github.com/RTradeLtd/grpc/krab"
	"github.com/RTradeLtd/kaas"
	"github.com/RTradeLtd/rtfs"
	"github.com/RTradeLtd/tns/tns"
)

var (
	// Version denotes the tag of this build
	Version string

	certFile = filepath.Join(os.Getenv("HOME"), "/certificates/api.pem")
	keyFile  = filepath.Join(os.Getenv("HOME"), "/certificates/api.key")
	tCfg     config.TemporalConfig
)

var commands = map[string]cmd.Cmd{
	"tns": {
		Blurb:         "run a tns daemon or client",
		Description:   "allows running a tns daemon to manage a zone, or a client to query a dameon",
		ChildRequired: true,
		Children: map[string]cmd.Cmd{
			"daemon": {
				Blurb:       "run tns daemon",
				Description: "runs a tns daemon and zone manager",
				Action: func(cfg config.TemporalConfig, args map[string]string) {
					kb, err := kaas.NewClient(cfg.Services, false)
					if err != nil {
						log.Fatal(err)
					}
					ctx, cancel := context.WithCancel(context.Background())
					resp, err := kb.GetPrivateKey(ctx, &pb.KeyGet{Name: args["managerKeyName"]})
					if err != nil {
						log.Fatal(err)
					}
					pk, err := ci.UnmarshalPrivateKey(resp.PrivateKey)
					if err != nil {
						log.Fatal(err)
					}
					dbm, err := database.New(&cfg, database.Options{})
					if err != nil {
						log.Fatal(err)
					}
					ipfs, err := rtfs.NewManager(
						cfg.IPFS.APIConnection.Host+":"+cfg.IPFS.APIConnection.Port,
						"", time.Minute*10,
					)
					if err != nil {
						log.Fatal(err)
					}
					daemon, err := tns.NewDaemon(ctx, &tns.Options{
						ManagerPK: pk,
						LogFile:   "/tmp/daemon.log",
						DB:        dbm.DB,
						IPFS:      ipfs,
						KBC:       kb,
					})
					if err != nil {
						log.Fatal(err)
					}
					// temporary, lazy to do anythign else atm
					go func() {
						time.Sleep(time.Minute * 1)
						cancel()
					}()
					if err := daemon.Run(ctx); err != nil {
						log.Fatal(err)
					}
				},
			},
			"client": {
				Blurb:       "run tns client",
				Description: "runs a tns client to make libp2p connections to a tns daemon",
				Action: func(cfg config.TemporalConfig, args map[string]string) {
					peerAddr := args["peerAddr"]
					if peerAddr == "" {
						log.Fatal("peerAddr argument is empty")
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					client, err := tns.NewClient(ctx, tns.ClientOptions{GenPK: true})
					if err != nil {
						log.Fatal(err)
					}
					defer client.Close()
					pid, err := client.H.AddPeer(peerAddr)
					if err != nil {
						log.Fatal(err)
					}
					if _, err = client.QueryTNS(pid, "echo", nil); err != nil {
						log.Fatal(err)
					}
				},
			},
		},
	},
	"migrate": {
		Blurb:       "run database migrations",
		Description: "Runs our initial database migrations, creating missing tables, etc..",
		Action: func(cfg config.TemporalConfig, args map[string]string) {
			if _, err := database.New(&cfg, database.Options{
				RunMigrations: true,
			}); err != nil {
				log.Fatal(err)
			}
		},
	},
	"migrate-insecure": {
		Hidden:      true,
		Blurb:       "run database migrations without SSL",
		Description: "Runs our initial database migrations, creating missing tables, etc.. without SSL",
		Action: func(cfg config.TemporalConfig, args map[string]string) {
			if _, err := database.New(&cfg, database.Options{
				RunMigrations:  true,
				SSLModeDisable: true,
			}); err != nil {
				log.Fatal(err)
			}
		},
	},
	"init": {
		PreRun:      true,
		Blurb:       "initialize blank Temporal configuration",
		Description: "Initializes a blank Temporal configuration template at CONFIG_DAG.",
		Action: func(cfg config.TemporalConfig, args map[string]string) {
			configDag := os.Getenv("CONFIG_DAG")
			if configDag == "" {
				log.Fatal("CONFIG_DAG is not set")
			}
			if err := config.GenerateConfig(configDag); err != nil {
				log.Fatal(err)
			}
		},
	},
}

func main() {
	// create app
	temporal := cmd.New(commands, cmd.Config{
		Name:     "Temporal",
		ExecName: "temporal",
		Version:  Version,
		Desc:     "Temporal is an easy-to-use interface into distributed and decentralized storage technologies for personal and enterprise use cases.",
	})
	managerKeyName := os.Getenv("MANAGER_KEY_NAME")
	if managerKeyName == "" {
		log.Fatal("MANAGER_KEY_NAME is not set")
	}
	// load arguments
	flags := map[string]string{
		"managerKeyName": managerKeyName,
	}
	// run no-config commands, exit if command was run
	if exit := temporal.PreRun(flags, os.Args[1:]); exit == cmd.CodeOK {
		os.Exit(0)
	}

	// load config
	configDag := os.Getenv("CONFIG_DAG")
	if configDag == "" {
		log.Fatal("CONFIG_DAG is not set")
	}
	tCfg, err := config.LoadConfig(configDag)
	if err != nil {
		log.Fatal(err)
	}

	// execute
	os.Exit(temporal.Run(*tCfg, flags, os.Args[1:]))
}
