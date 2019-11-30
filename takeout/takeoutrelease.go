package takeout

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshui "github.com/cloudfoundry/bosh-cli/ui"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"gopkg.in/yaml.v2"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var BadChar = regexp.MustCompile("[?=\"]")

type RealUtensils struct {
}

func (c RealUtensils) Download(url string, localFileName string) (result DownloadInfo, err error) {

	tempFileName := localFileName + ".download"
	resp, err := http.Get(url)

	if resp != nil {
		defer func() {
			if ferr := resp.Body.Close(); ferr != nil {
				err = ferr
			}
		}()
	}
	if err != nil {
		return DownloadInfo{}, err
	}

	// Create the file
	out, err := os.Create(tempFileName)
	if out != nil {
		defer func() {
			if ferr := out.Close(); ferr != nil {
				err = ferr
			}
		}()
	}
	if err != nil {
		return DownloadInfo{}, err
	}

	// Write the body to file
	hashSha1 := sha1.New()
	hashSha256 := sha256.New()
	_, err = io.Copy(out, io.TeeReader(io.TeeReader(resp.Body, hashSha1), hashSha256))
	actualSha1 := fmt.Sprintf("%x", hashSha1.Sum(nil))
	actualSha256 := fmt.Sprintf("%x", hashSha256.Sum(nil))
	if err != nil {
		return DownloadInfo{}, err
	}
	err = os.Rename(tempFileName, localFileName)
	if err != nil {
		return DownloadInfo{}, err
	}

	return DownloadInfo{
		sha1:     actualSha1,
		sha256:   actualSha256,
		fileName: localFileName,
	}, nil
}

func (c RealUtensils) TakeOutStemcell(s boshdir.ManifestReleaseStemcell, ui boshui.UI, stemCellType string) (err error) {

	localFileName := fmt.Sprintf("bosh-%s-%s-go_agent-stemcell_v%s.tgz", stemCellType, s.OS, s.Version)

	if _, err := os.Stat(localFileName); os.IsNotExist(err) {

		url := fmt.Sprintf("https://bosh.io/d/stemcells/bosh-%s-%s-go_agent?v=%s", stemCellType, s.OS, s.Version)
		result, err := c.Download(url, localFileName)
		if err != nil {
			return err
		}
		ui.PrintLinef("Stemcell downloaded %s", result.fileName)

	} else {
		ui.PrintLinef("Stemcell present: %s", localFileName)
	}
	return
}

func (c RealUtensils) TakeOutRelease(r boshdir.ManifestRelease, ui boshui.UI) (entry OpEntry, err error) {

	// generate a local file name that's safe
	localFileName := BadChar.ReplaceAllString(fmt.Sprintf("%s_v%s.tgz", r.Name, r.Version), "_")

	if _, err := os.Stat(localFileName); os.IsNotExist(err) {
		ui.PrintLinef("Downloading release: %s / %s -> %s", r.Name, r.Version, localFileName)

		result, err := c.Download(r.URL, localFileName)
		if err != nil {
			return OpEntry{}, err
		}
		if len(r.SHA1) == 40 {
			if result.sha1 != r.SHA1 {
				return OpEntry{}, bosherr.Errorf("sha1 mismatch %s (a:%s, e:%s)", localFileName, result.sha1, r.SHA1)
			}
		} else if strings.HasPrefix(r.SHA1, "sha256") {
			expected := strings.Split(r.SHA1, ":")[1]
			if result.sha256 != expected {
				return OpEntry{}, bosherr.Errorf("sha256 mismatch %s (a:%s, e:%s)", localFileName, result.sha256, expected)
			}
		}
	} else {
		ui.PrintLinef("Release present: %s / %s -> %s", r.Name, r.Version, localFileName)
	}
	if len(r.Name) > 0 {
		path := fmt.Sprintf("/releases/name=%s/url", r.Name)
		entry = OpEntry{Type: "remove", Path: path}
	}
	return entry, err
}

func (c RealUtensils) ParseDeployment(bytes []byte) (Manifest, error) {
	var deployment Manifest

	err := yaml.Unmarshal(bytes, &deployment)
	if err != nil {
		return deployment, err
	}

	return deployment, nil
}
