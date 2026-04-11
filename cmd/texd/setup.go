package main

import (
	"context"

	"github.com/digineo/texd/exec"
	"github.com/digineo/texd/refstore"
	"github.com/digineo/texd/refstore/nop"
	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/docker/go-units"
	"github.com/digineo/texd/xlog"
)

// configureTeX sets up the tex package globals based on config.
func configureTeX(cfg *config, log xlog.Logger) error {
	if err := tex.SetJobBaseDir(cfg.jobDir); err != nil {
		log.Error("error setting job directory",
			xlog.String("flag", "--job-directory"),
			xlog.Error(err))
		return err
	}

	if err := tex.SetDefaultEngine(cfg.engine); err != nil {
		log.Error("error setting default TeX engine",
			xlog.String("flag", "--tex-engine"),
			xlog.Error(err))
		return err
	}

	// Handle shell escaping tri-state: 0=default, 1=enable, -1=disable
	if cfg.shellEscape != 0 {
		if cfg.shellEscape > 0 {
			_ = tex.SetShellEscaping(tex.AllowedShellEscape)
		} else {
			_ = tex.SetShellEscaping(tex.ForbiddenShellEscape)
		}
	}

	return nil
}

// buildServiceOptions creates service.Options from config.
func buildServiceOptions(cfg *config, log xlog.Logger) (service.Options, error) {
	opts := service.Options{
		Addr:           cfg.addr,
		QueueLength:    cfg.queueLength,
		QueueTimeout:   cfg.queueTimeout,
		CompileTimeout: cfg.compileTimeout,
		Mode:           "local",
		Executor:       exec.LocalExec,
		KeepJobs:       cfg.keepJobs,
	}

	// Parse and set max job size
	if maxsz, err := units.FromHumanSize(cfg.maxJobSize); err != nil {
		log.Error("error parsing maximum job size",
			xlog.String("flag", "--max-job-size"),
			xlog.Error(err))
		return opts, err
	} else {
		opts.MaxJobSize = maxsz
	}

	// Setup reference store if configured
	if cfg.storageDSN != "" {
		rp, err := createRetentionPolicy(cfg.retPolicy, cfg.retPolItems, cfg.retPolSize)
		if err != nil {
			log.Error("error initializing retention policy",
				xlog.String("flag", "--retention-policy, and/or --rp-access-items, --rp-access-size"),
				xlog.Error(err))
			return opts, err
		}
		adapter, err := refstore.NewStore(cfg.storageDSN, rp)
		if err != nil {
			log.Error("error parsing reference store DSN",
				xlog.String("flag", "--reference-store"),
				xlog.Error(err))
			return opts, err
		}
		opts.RefStore = adapter
	} else {
		opts.RefStore, _ = nop.New(nil, nil)
	}

	// Setup Docker executor if images specified
	if len(cfg.images) > 0 {
		log.Info("using docker", xlog.Any("images", cfg.images))
		cli, err := exec.NewDockerClient(log, tex.JobBaseDir())
		if err != nil {
			log.Error("error connecting to dockerd", xlog.Error(err))
			return opts, err
		}

		opts.Images, err = cli.SetImages(context.Background(), cfg.pull, cfg.images...)
		if err != nil {
			log.Error("error setting images", xlog.Error(err))
			return opts, err
		}
		opts.Mode = "container"
		opts.Executor = cli.Executor
	}

	return opts, nil
}

// createRetentionPolicy creates a retention policy based on the given parameters.
func createRetentionPolicy(policy int, items int, size string) (refstore.RetentionPolicy, error) {
	switch policy {
	case 0:
		return &refstore.KeepForever{}, nil
	case 1:
		return &refstore.PurgeOnStart{}, nil
	case 2: //nolint:mnd
		sz, err := units.FromHumanSize(size)
		if err != nil {
			return nil, err
		}
		pol, err := refstore.NewAccessList(items, int(sz))
		if err != nil {
			return nil, err
		}
		return pol, nil
	}
	panic("not reached")
}
