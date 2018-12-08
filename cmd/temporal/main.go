package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	pb "github.com/RTradeLtd/grpc/krab"
	"github.com/RTradeLtd/kaas"
	"github.com/RTradeLtd/tns/tns"
	ci "github.com/libp2p/go-libp2p-crypto"

	"github.com/RTradeLtd/cmd"
	"github.com/RTradeLtd/config"
	"github.com/RTradeLtd/database"
	"github.com/RTradeLtd/database/models"
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
					kb, err := kaas.NewClient(cfg.Endpoints)
					if err != nil {
						log.Fatal(err)
					}
					resp, err := kb.GetPrivateKey(context.Background(), &pb.KeyGet{Name: cfg.TNS.ZoneManagerKeyName})
					if err != nil {
						log.Fatal(err)
					}
					zoneManagerPK, err := ci.UnmarshalPrivateKey(resp.PrivateKey)
					if err != nil {
						log.Fatal(err)
					}
					resp, err = kb.GetPrivateKey(context.Background(), &pb.KeyGet{Name: cfg.TNS.ZoneKeyName})
					if err != nil {
						log.Fatal(err)
					}
					zonePK, err := ci.UnmarshalPrivateKey(resp.PrivateKey)
					if err != nil {
						log.Fatal(err)
					}
					managerOpts := tns.ManagerOpts{
						ManagerPK: zoneManagerPK,
						ZonePK:    zonePK,
						ZoneName:  cfg.TNS.ZoneName,
					}
					dbm, err := database.Initialize(&cfg, database.Options{})
					if err != nil {
						log.Fatal(err)
					}
					manager, err := tns.GenerateTNSManager(&managerOpts, dbm.DB)
					if err != nil {
						log.Fatal(err)
					}
					if err = manager.MakeHost(manager.PrivateKey, nil); err != nil {
						log.Fatal(err)
					}
					defer manager.Host.Close()
					manager.RunTNSDaemon()
					lim := len(manager.Host.Addrs())
					count := 0
					for count < lim {
						fmt.Println(manager.ReachableAddress(count))
						count++
					}
					select {}
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
					client, err := tns.GenerateTNSClient(true, nil)
					if err != nil {
						log.Fatal(err)
					}
					if err = client.MakeHost(client.PrivateKey, nil); err != nil {
						log.Fatal(err)
					}
					defer client.Host.Close()
					pid, err := client.AddPeerToPeerStore(peerAddr)
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
			if _, err := database.Initialize(&cfg, database.Options{
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
			if _, err := database.Initialize(&cfg, database.Options{
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
	"user": {
		Hidden:      true,
		Blurb:       "create a user",
		Description: "Create a Temporal user. Provide args as username, password, email. Do not use in production.",
		Action: func(cfg config.TemporalConfig, args map[string]string) {
			if len(os.Args) < 5 {
				log.Fatal("insufficient fields provided")
			}
			d, err := database.Initialize(&cfg, database.Options{
				SSLModeDisable: true,
			})
			if err != nil {
				log.Fatal(err)
			}
			if _, err := models.NewUserManager(d.DB).NewUserAccount(
				os.Args[2], os.Args[3], os.Args[4], false,
			); err != nil {
				log.Fatal(err)
			}
		},
	},
	"admin": {
		Hidden:      true,
		Blurb:       "assign user as an admin",
		Description: "Assign an existing Temporal user as an administrator.",
		Action: func(cfg config.TemporalConfig, args map[string]string) {
			if len(os.Args) < 3 {
				log.Fatal("no user provided")
			}
			d, err := database.Initialize(&cfg, database.Options{
				SSLModeDisable: true,
			})
			if err != nil {
				log.Fatal(err)
			}
			found, err := models.NewUserManager(d.DB).ToggleAdmin(os.Args[2])
			if err != nil {
				log.Fatal(err)
			}
			if !found {
				log.Fatalf("user %s not found", os.Args[2])
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

	// run no-config commands, exit if command was run
	if exit := temporal.PreRun(os.Args[1:]); exit == cmd.CodeOK {
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
	// load arguments
	flags := map[string]string{
		"configDag":     configDag,
		"certFilePath":  tCfg.API.Connection.Certificates.CertPath,
		"keyFilePath":   tCfg.API.Connection.Certificates.KeyPath,
		"listenAddress": tCfg.API.Connection.ListenAddress,

		"dbPass": tCfg.Database.Password,
		"dbURL":  tCfg.Database.URL,
		"dbUser": tCfg.Database.Username,
	}
	var (
		peerAddr string
		isTns    bool
	)
	// check for tns client operation and load peer addr
	for _, v := range os.Args {
		if v == "tns" {
			isTns = true
		}
		if isTns && v == "client" {
			peerAddr = os.Getenv("PEER_ADDR")
			if peerAddr == "" {
				log.Fatal("PEER_ADDR env var is empty")
			}
		}
	}
	if isTns && peerAddr != "" {
		flags["peerAddr"] = peerAddr
	}
	fmt.Println(tCfg.APIKeys.ChainRider)
	// execute
	os.Exit(temporal.Run(*tCfg, flags, os.Args[1:]))
}
