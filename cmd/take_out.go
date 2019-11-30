package cmd

import (
	. "github.com/cloudfoundry/bosh-cli/cmd/opts"
	boshtpl "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/cloudfoundry/bosh-cli/takeout"
	boshui "github.com/cloudfoundry/bosh-cli/ui"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"gopkg.in/yaml.v2"
	"os"
)

type TakeOutCmd struct {
	ui boshui.UI
	to takeout.Utensils
}

func NewTakeOutCmd(ui boshui.UI, d takeout.Utensils) TakeOutCmd {
	return TakeOutCmd{ui: ui, to: d}
}

func (c TakeOutCmd) Run(opts TakeOutOpts) error {
	tpl := boshtpl.NewTemplate(opts.Args.Manifest.Bytes)

	bytes, err := tpl.Evaluate(opts.VarFlags.AsVariables(), opts.OpsFlags.AsOp(), boshtpl.EvaluateOpts{})
	if err != nil {
		return bosherr.WrapErrorf(err, "Evaluating manifest")
	}

	if _, err := os.Stat(opts.Args.Name); os.IsExist(err) {
		c.ui.PrintLinef("ops file already exists, will be overwritten")
	}
	deployment, err := c.to.ParseDeployment(bytes)

	if err != nil {
		return bosherr.WrapError(err, "Problem parsing deployment")
	}
	c.ui.PrintLinef("Processing releases for offline use")
	var releaseChanges []takeout.OpEntry
	for _, r := range deployment.Releases {
		if r.URL == "" {
			c.ui.PrintLinef("Release does not have a URL for take-out; Name: %s / %s", r.Name, r.Version)
			return bosherr.WrapErrorf(nil, "Provide an opsfile that has a URL or remove this release")
		} else {
			o, err := c.to.TakeOutRelease(r, c.ui)
			if err != nil {
				return err
			}
			releaseChanges = append(releaseChanges, o)
		}
	}
	for _, s := range deployment.Stemcells {
		if s.Version != "latest" {
			err := c.to.TakeOutStemcell(s, c.ui, opts.StemcellType)
			if err != nil {
				return bosherr.WrapErrorf(err, "Failed to get stemcell")
			}
		}
	}

	y, _ := yaml.Marshal(releaseChanges)
	c.ui.PrintLinef("Writing take_out operation to file: " + opts.Args.Name)
	takeoutOp, err := os.Create(opts.Args.Name)
	if err != nil {
		return err
	}

	_, err = takeoutOp.WriteString("---\n")
	_, err = takeoutOp.WriteString(string(y))
	if takeoutOp != nil {
		defer func() {
			if ferr := takeoutOp.Close(); ferr != nil {
				err = ferr
			}
		}()
	}

	if err != nil {
		return err
	}
	return nil
}
