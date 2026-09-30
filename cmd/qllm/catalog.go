package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"qLLM/internal/cataloggen"
	"qLLM/internal/config"
	"qLLM/internal/protocol"

	"github.com/spf13/cobra"
)

// newCatalogCmd implements runtime behavior for this package.
func newCatalogCmd(opts func() config.Options, addFlags func(*cobra.Command)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Author catalog YAML (introspect SQL or import OpenAPI). Writes files; does not serve.",
	}
	cmd.AddCommand(newIntrospectCmd(opts, addFlags), newFromOpenAPICmd(opts, addFlags))
	return cmd
}

// newIntrospectCmd implements runtime behavior for this package.
func newIntrospectCmd(opts func() config.Options, addFlags func(*cobra.Command)) *cobra.Command {
	var source, out string
	var merge bool
	cmd := &cobra.Command{
		Use:   "introspect",
		Short: "Read information_schema from a postgres/mysql source and write draft catalog YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return fmt.Errorf("--source is required")
			}
			cfg := opts()
			if err := config.ApplyEnvFile(cfg.ConfigDir); err != nil {
				return printErr(err)
			}
			p, c, _, err := config.LoadBundle(cfg)
			if err != nil {
				return printErr(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			ents, err := cataloggen.IntrospectSQL(ctx, p, source, p.Limits.MaxSourceMs)
			if err != nil {
				return printErr(err)
			}
			outCat := &protocol.Catalog{
				ProtocolVersion: protocol.ProtocolVersion,
				Project:         p.Project,
				Entities:        ents,
			}
			if merge {
				outCat = cataloggen.MergeSourceEntities(c, source, ents)
				if outCat.Project == "" {
					outCat.Project = p.Project
				}
			}
			raw, err := cataloggen.EncodeCatalog(outCat)
			if err != nil {
				return err
			}
			return writeOut(out, raw)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "preset source id (postgres or mysql)")
	cmd.Flags().StringVar(&out, "out", "", "output catalog YAML path (default stdout)")
	cmd.Flags().BoolVar(&merge, "merge", false, "keep entities from other sources in the loaded catalog")
	addFlags(cmd)
	return cmd
}

// newFromOpenAPICmd implements runtime behavior for this package.
func newFromOpenAPICmd(opts func() config.Options, addFlags func(*cobra.Command)) *cobra.Command {
	var source, specPath, out, resourcesOut string
	var merge bool
	cmd := &cobra.Command{
		Use:   "from-openapi",
		Short: "Generate rest_resource entities and options.resources from an OpenAPI 3 file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if specPath == "" {
				return fmt.Errorf("-f / --file is required")
			}
			if source == "" {
				return fmt.Errorf("--source is required")
			}
			cfg := opts()
			if err := config.ApplyEnvFile(cfg.ConfigDir); err != nil {
				return printErr(err)
			}
			p, c, _, err := config.LoadBundle(cfg)
			if err != nil {
				return printErr(err)
			}
			var srcOK bool
			for _, s := range p.Sources {
				if s.ID == source {
					srcOK = true
					if s.Type != protocol.SourceREST {
						return printErr(protocol.NewError(protocol.ErrConfigError,
							"source "+source+" is not type rest", nil))
					}
					break
				}
			}
			if !srcOK {
				return printErr(protocol.NewError(protocol.ErrConfigError, "unknown source: "+source, nil))
			}
			spec, err := os.ReadFile(specPath)
			if err != nil {
				return err
			}
			gen, err := cataloggen.FromOpenAPI(spec, source)
			if err != nil {
				return printErr(err)
			}
			outCat := &protocol.Catalog{
				ProtocolVersion: protocol.ProtocolVersion,
				Project:         p.Project,
				Entities:        gen.Entities,
			}
			if merge {
				outCat = cataloggen.MergeSourceEntities(c, source, gen.Entities)
				if outCat.Project == "" {
					outCat.Project = p.Project
				}
			}
			raw, err := cataloggen.EncodeCatalog(outCat)
			if err != nil {
				return err
			}
			if err := writeOut(out, raw); err != nil {
				return err
			}
			rb, err := cataloggen.EncodeResources(gen.Resources)
			if err != nil {
				return err
			}
			if resourcesOut != "" {
				return cataloggen.WriteFile(resourcesOut, rb)
			}
			fmt.Fprintln(os.Stderr, "--- resources (paste under sources[].options.resources) ---")
			_, _ = os.Stderr.Write(rb)
			return nil
		},
	}
	cmd.Flags().StringVarP(&specPath, "file", "f", "", "OpenAPI 3 YAML/JSON file")
	cmd.Flags().StringVar(&source, "source", "", "REST source id in the preset")
	cmd.Flags().StringVar(&out, "out", "", "output catalog YAML path (default stdout)")
	cmd.Flags().StringVar(&resourcesOut, "resources-out", "", "write options.resources YAML fragment")
	cmd.Flags().BoolVar(&merge, "merge", false, "keep entities from other sources in the loaded catalog")
	addFlags(cmd)
	return cmd
}

// writeOut implements runtime behavior for this package.
func writeOut(path string, raw []byte) error {
	if path == "" {
		_, err := os.Stdout.Write(raw)
		return err
	}
	return cataloggen.WriteFile(path, raw)
}
