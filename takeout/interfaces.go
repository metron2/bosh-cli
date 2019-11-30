package takeout

import (
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshui "github.com/cloudfoundry/bosh-cli/ui"
)

type Utensils interface {
	DeploymentReader
	Release
	Stemcell
	Downloader
}
type Manifest struct {
	Name      string
	Releases  []boshdir.ManifestRelease
	Stemcells []boshdir.ManifestReleaseStemcell
}
type DownloadInfo struct {
	sha1     string
	sha256   string
	fileName string
}
type Downloader interface {
	Download(url string, localFileName string) (result DownloadInfo, err error)
}

type DeploymentReader interface {
	ParseDeployment(bytes []byte) (Manifest, error)
}

type Stemcell interface {
	TakeOutStemcell(s boshdir.ManifestReleaseStemcell, ui boshui.UI, stemCellType string) (err error)
}

type Release interface {
	TakeOutRelease(r boshdir.ManifestRelease, ui boshui.UI) (entry OpEntry, err error)
}
