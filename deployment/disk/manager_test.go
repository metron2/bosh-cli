package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
	fakeuuid "github.com/cloudfoundry/bosh-agent/uuid/fakes"
	fakebmcloud "github.com/cloudfoundry/bosh-micro-cli/cloud/fakes"
	fakebmlog "github.com/cloudfoundry/bosh-micro-cli/eventlogger/fakes"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	bmconfig "github.com/cloudfoundry/bosh-micro-cli/config"
	bmdisk "github.com/cloudfoundry/bosh-micro-cli/deployment/disk"
	bmdeplmanifest "github.com/cloudfoundry/bosh-micro-cli/deployment/manifest"
	bmeventlog "github.com/cloudfoundry/bosh-micro-cli/eventlogger"

	. "github.com/cloudfoundry/bosh-micro-cli/deployment/disk"
)

var _ = Describe("Manager", func() {
	var (
		manager           Manager
		fakeCloud         *fakebmcloud.FakeCloud
		fakeFs            *fakesys.FakeFileSystem
		fakeUUIDGenerator *fakeuuid.FakeGenerator
		diskRepo          bmconfig.DiskRepo
	)

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fakeFs = fakesys.NewFakeFileSystem()
		fakeUUIDGenerator = &fakeuuid.FakeGenerator{}
		configService := bmconfig.NewFileSystemDeploymentConfigService("/fake/path", fakeFs, fakeUUIDGenerator, logger)
		diskRepo = bmconfig.NewDiskRepo(configService, fakeUUIDGenerator)
		managerFactory := NewManagerFactory(diskRepo, logger)
		fakeCloud = fakebmcloud.NewFakeCloud()
		manager = managerFactory.NewManager(fakeCloud)
		fakeUUIDGenerator.GeneratedUuid = "fake-uuid"
	})

	Describe("Create", func() {
		var (
			diskPool bmdeplmanifest.DiskPool
		)

		BeforeEach(func() {

			diskPool = bmdeplmanifest.DiskPool{
				Name:     "fake-disk-pool-name",
				DiskSize: 1024,
				RawCloudProperties: map[interface{}]interface{}{
					"fake-cloud-property-key": "fake-cloud-property-value",
				},
			}
		})

		Context("when creating disk succeeds", func() {
			BeforeEach(func() {
				fakeCloud.CreateDiskCID = "fake-disk-cid"
			})

			It("returns a disk", func() {
				disk, err := manager.Create(diskPool, "fake-vm-cid")
				Expect(err).ToNot(HaveOccurred())
				Expect(disk.CID()).To(Equal("fake-disk-cid"))
			})

			It("saves the disk record", func() {
				_, err := manager.Create(diskPool, "fake-vm-cid")
				Expect(err).ToNot(HaveOccurred())

				diskRecord, found, err := diskRepo.Find("fake-disk-cid")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(diskRecord).To(Equal(bmconfig.DiskRecord{
					ID:   "fake-uuid",
					CID:  "fake-disk-cid",
					Size: 1024,
					CloudProperties: map[string]interface{}{
						"fake-cloud-property-key": "fake-cloud-property-value",
					},
				}))
			})
		})

		Context("when creating disk fails", func() {
			BeforeEach(func() {
				fakeCloud.CreateDiskErr = errors.New("fake-create-error")
			})

			It("returns an error", func() {
				_, err := manager.Create(diskPool, "fake-vm-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-create-error"))
			})
		})

		Context("when updating disk record fails", func() {
			BeforeEach(func() {
				fakeFs.WriteToFileError = errors.New("fake-write-error")
			})

			It("returns an error", func() {
				_, err := manager.Create(diskPool, "fake-vm-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-write-error"))
			})
		})
	})

	Describe("FindCurrent", func() {
		Context("when disk already exists in disk repo", func() {
			BeforeEach(func() {
				diskRecord, err := diskRepo.Save("fake-existing-disk-cid", 1024, map[string]interface{}{})
				Expect(err).ToNot(HaveOccurred())

				err = diskRepo.UpdateCurrent(diskRecord.ID)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the existing disk", func() {
				disks, err := manager.FindCurrent()
				Expect(err).ToNot(HaveOccurred())
				Expect(disks).To(HaveLen(1))
				Expect(disks[0].CID()).To(Equal("fake-existing-disk-cid"))
			})
		})

		Context("when disk does not exists in disk repo", func() {
			It("returns an empty array", func() {
				disks, err := manager.FindCurrent()
				Expect(err).ToNot(HaveOccurred())
				Expect(disks).To(BeEmpty())
			})
		})

		Context("when reading disk repo fails", func() {
			BeforeEach(func() {
				fakeFs.WriteFileString("/fake/path", "{}")
				fakeFs.ReadFileError = errors.New("fake-read-error")
			})

			It("returns an error", func() {
				_, err := manager.FindCurrent()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-read-error"))
			})
		})
	})

	Describe("FindUnused", func() {
		var (
			firstDisk bmdisk.Disk
			thirdDisk bmdisk.Disk
		)

		BeforeEach(func() {
			fakeUUIDGenerator.GeneratedUuid = "fake-guid-1"
			firstDiskRecord, err := diskRepo.Save("fake-disk-cid-1", 1024, map[string]interface{}{})
			Expect(err).ToNot(HaveOccurred())
			firstDisk = NewDisk(firstDiskRecord, fakeCloud, diskRepo)

			fakeUUIDGenerator.GeneratedUuid = "fake-guid-2"
			_, err = diskRepo.Save("fake-disk-cid-2", 1024, map[string]interface{}{})
			Expect(err).ToNot(HaveOccurred())
			err = diskRepo.UpdateCurrent("fake-guid-2")
			Expect(err).ToNot(HaveOccurred())

			fakeUUIDGenerator.GeneratedUuid = "fake-guid-3"
			thirdDiskRecord, err := diskRepo.Save("fake-disk-cid-3", 1024, map[string]interface{}{})
			Expect(err).ToNot(HaveOccurred())
			thirdDisk = NewDisk(thirdDiskRecord, fakeCloud, diskRepo)
		})

		It("returns unused disks from repo", func() {
			disks, err := manager.FindUnused()
			Expect(err).ToNot(HaveOccurred())

			Expect(disks).To(Equal([]bmdisk.Disk{
				firstDisk,
				thirdDisk,
			}))
		})
	})

	Describe("DeleteUnused", func() {
		var (
			secondDiskRecord bmconfig.DiskRecord
			fakeStage        *fakebmlog.FakeStage
		)
		BeforeEach(func() {
			fakeStage = fakebmlog.NewFakeStage()

			fakeUUIDGenerator.GeneratedUuid = "fake-disk-id-1"
			_, err := diskRepo.Save("fake-disk-cid-1", 100, nil)
			Expect(err).ToNot(HaveOccurred())

			fakeUUIDGenerator.GeneratedUuid = "fake-disk-id-2"
			secondDiskRecord, err = diskRepo.Save("fake-disk-cid-2", 100, nil)
			Expect(err).ToNot(HaveOccurred())
			err = diskRepo.UpdateCurrent(secondDiskRecord.ID)
			Expect(err).ToNot(HaveOccurred())

			fakeUUIDGenerator.GeneratedUuid = "fake-disk-id-3"
			_, err = diskRepo.Save("fake-disk-cid-3", 100, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("deletes unused disks", func() {
			err := manager.DeleteUnused(fakeStage)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeCloud.DeleteDiskInputs).To(Equal([]fakebmcloud.DeleteDiskInput{
				{DiskCID: "fake-disk-cid-1"},
				{DiskCID: "fake-disk-cid-3"},
			}))

			Expect(fakeStage.Steps).To(ContainElement(&fakebmlog.FakeStep{
				Name: "Deleting unused disk 'fake-disk-cid-1'",
				States: []bmeventlog.EventState{
					bmeventlog.Started,
					bmeventlog.Finished,
				},
			}))
			Expect(fakeStage.Steps).To(ContainElement(&fakebmlog.FakeStep{
				Name: "Deleting unused disk 'fake-disk-cid-3'",
				States: []bmeventlog.EventState{
					bmeventlog.Started,
					bmeventlog.Finished,
				},
			}))

			currentRecord, found, err := diskRepo.FindCurrent()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(currentRecord).To(Equal(secondDiskRecord))

			records, err := diskRepo.All()
			Expect(err).ToNot(HaveOccurred())
			Expect(records).To(Equal([]bmconfig.DiskRecord{
				secondDiskRecord,
			}))
		})
	})
})