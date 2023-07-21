package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/overmindtech/discovery"
	"github.com/overmindtech/sdp-go/auth"
	"github.com/overmindtech/source-template/sources"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "source-template",
	Short: "Remote primary source for some technology",
	Long: `A template for building sources.

Edit this once you have created your source
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get srcman supplied config
		natsServers := viper.GetStringSlice("nats-servers")
		natsNamePrefix := viper.GetString("nats-name-prefix")
		maxParallel := viper.GetInt("max-parallel")
		natsJWT := viper.GetString("nats-jwt")
		natsNKeySeed := viper.GetString("nats-nkey-seed")
		hostname, err := os.Hostname()

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("Could not determine hostname for use in NATS connection name")
		}

		// ⚠️ Your custom configration goes here
		yourCustomFlag := viper.GetString("your-custom-flag")

		var natsNKeySeedLog string
		var tokenClient auth.TokenClient

		if natsNKeySeed != "" {
			natsNKeySeedLog = "[REDACTED]"
		}

		log.WithFields(log.Fields{
			"nats-servers":     natsServers,
			"nats-name-prefix": natsNamePrefix,
			"max-parallel":     maxParallel,
			"nats-jwt":         natsJWT,
			"nats-nkey-seed":   natsNKeySeedLog,
			"your-custom-flag": yourCustomFlag,
		}).Info("Got config")

		// Validate the auth params and create a token client if we are using
		// auth
		if natsJWT != "" || natsNKeySeed != "" {
			var err error

			tokenClient, err = createTokenClient(natsJWT, natsNKeySeed)

			if err != nil {
				log.WithFields(log.Fields{
					"error": err.Error(),
				}).Fatal("Error validating authentication info")
			}
		}

		e, err := discovery.NewEngine()
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Fatal("Error initializing Engine")
		}
		e.Name = "source-template"
		e.NATSOptions = &auth.NATSOptions{
			NumRetries:        -1,
			RetryDelay:        5 * time.Second,
			Servers:           natsServers,
			ConnectionName:    fmt.Sprintf("%v.%v", natsNamePrefix, hostname),
			ConnectionTimeout: (10 * time.Second), // TODO: Make configurable
			MaxReconnects:     999,                // We are in a container so wait forever
			ReconnectWait:     2 * time.Second,
			ReconnectJitter:   2 * time.Second,
			TokenClient:       tokenClient,
		}
		e.NATSQueueName = "source-template" // This should be the same as your engine name
		e.MaxParallelExecutions = maxParallel

		// ⚠️ Here is where you add your sources
		colourNameSource := sources.ColourNameSource{}

		e.AddSources(&colourNameSource)

		// Start HTTP server for status
		healthCheckPort := 8080
		healthCheckPath := "/healthz"

		http.HandleFunc(healthCheckPath, func(rw http.ResponseWriter, r *http.Request) {
			if e.IsNATSConnected() {
				fmt.Fprint(rw, "ok")
			} else {
				http.Error(rw, "NATS not connected", http.StatusInternalServerError)
			}
		})

		log.WithFields(log.Fields{
			"port": healthCheckPort,
			"path": healthCheckPath,
		}).Debug("Starting healthcheck server")

		go func() {
			err := http.ListenAndServe(":8080", nil)

			if err != nil {
				log.WithError(err).Fatal("Could not start HTTP server for /healthz health checks")
			}
		}()

		err = e.Start()

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("Could not start engine")
		}

		sigs := make(chan os.Signal, 1)

		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs

		log.Info("Stopping engine")

		err = e.Stop()

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("Could not stop engine")
		}

		log.Info("Stopped")

		os.Exit(0)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	var logLevel string

	// General config options
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/etc/srcman/config/source.yaml", "config file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log", "info", "Set the log level. Valid values: panic, fatal, error, warn, info, debug, trace")

	// Config required by all sources in order to connect to NATS. You shouldn't
	// need to change these
	rootCmd.PersistentFlags().StringArray("nats-servers", []string{"nats://localhost:4222", "nats://nats:4222"}, "A list of NATS servers to connect to")
	rootCmd.PersistentFlags().String("nats-name-prefix", "", "A name label prefix. Sources should append a dot and their hostname .{hostname} to this, then set this is the NATS connection name which will be sent to the server on CONNECT to identify the client")
	rootCmd.PersistentFlags().String("nats-jwt", "", "The JWT token that should be used to authenticate to NATS, provided in raw format e.g. eyJ0eXAiOiJKV1Q...")
	rootCmd.PersistentFlags().String("nats-nkey-seed", "", "The NKey seed which corresponds to the NATS JWT e.g. SUAFK6QUC...")
	rootCmd.PersistentFlags().Int("max-parallel", (runtime.NumCPU() * 2), "Max number of requests to run in parallel")

	// ⚠️ Add your own custom config options below, the example "your-custom-flag"
	// should be replaced with your own config or deleted
	rootCmd.PersistentFlags().String("your-custom-flag", "someDefaultValue.conf", "Description of what your option is meant to do")

	// Bind these to viper
	viper.BindPFlags(rootCmd.PersistentFlags())

	// Run this before we do anything to set up the loglevel
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if lvl, err := log.ParseLevel(logLevel); err == nil {
			log.SetLevel(lvl)
		} else {
			log.SetLevel(log.InfoLevel)
		}

		log.AddHook(TerminationLogHook{})

		// Bind flags that haven't been set to the values from viper of we have them
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			// Bind the flag to viper only if it has a non-empty default
			if f.DefValue != "" || f.Changed {
				viper.BindPFlag(f.Name, f)
			}
		})
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigFile(cfgFile)

	replacer := strings.NewReplacer("-", "_")

	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Infof("Using config file: %v", viper.ConfigFileUsed())
	}
}

// createTokenClient Creates a basic token client that will authenticate to NATS
// using the given values
func createTokenClient(natsJWT string, natsNKeySeed string) (auth.TokenClient, error) {
	var kp nkeys.KeyPair
	var err error

	if natsJWT == "" {
		return nil, errors.New("nats-jwt was blank. This is required when using authentication")
	}

	if natsNKeySeed == "" {
		return nil, errors.New("nats-nkey-seed was blank. This is required when using authentication")
	}

	if _, err = jwt.DecodeUserClaims(natsJWT); err != nil {
		return nil, fmt.Errorf("could not parse nats-jwt: %v", err)
	}

	if kp, err = nkeys.FromSeed([]byte(natsNKeySeed)); err != nil {
		return nil, fmt.Errorf("could not parse nats-nkey-seed: %v", err)
	}

	return auth.NewBasicTokenClient(natsJWT, kp), nil
}

// TerminationLogHook A hook that logs fatal errors to the termination log
type TerminationLogHook struct{}

func (t TerminationLogHook) Levels() []log.Level {
	return []log.Level{log.FatalLevel}
}

func (t TerminationLogHook) Fire(e *log.Entry) error {
	tLog, err := os.OpenFile("/dev/termination-log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	var message string

	message = e.Message

	for k, v := range e.Data {
		message = fmt.Sprintf("%v %v=%v", message, k, v)
	}

	_, err = tLog.WriteString(message)

	return err
}
