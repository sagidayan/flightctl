package store_test

import (
	"context"
	"testing"

	api "github.com/flightctl/flightctl/api/v1alpha1"
	"github.com/flightctl/flightctl/internal/config"
	"github.com/flightctl/flightctl/internal/store"
	"github.com/flightctl/flightctl/internal/store/model"
	"github.com/flightctl/flightctl/internal/util"
	flightlog "github.com/flightctl/flightctl/pkg/log"
	testutil "github.com/flightctl/flightctl/test/util"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func TestStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Store Suite")
}

var _ = Describe("DeviceStore create", func() {
	var (
		log        *logrus.Logger
		ctx        context.Context
		orgId      uuid.UUID
		storeInst  store.Store
		devStore   store.Device
		cfg        *config.Config
		dbName     string
		numDevices int
		called     bool
		callback   store.DeviceStoreCallback
	)

	BeforeEach(func() {
		ctx = context.Background()
		orgId, _ = uuid.NewUUID()
		log = flightlog.InitLogs()
		numDevices = 3
		storeInst, cfg, dbName = store.PrepareDBForUnitTests(log)
		devStore = storeInst.Device()
		called = false
		callback = store.DeviceStoreCallback(func(before *model.Device, after *model.Device) { called = true })

		testutil.CreateTestDevices(3, ctx, devStore, orgId, nil, false)
	})

	AfterEach(func() {
		store.DeleteTestDB(cfg, storeInst, dbName)
	})

	Context("Device store", func() {
		It("Get device success", func() {
			dev, err := devStore.Get(ctx, orgId, "mydevice-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(*dev.Metadata.Name).To(Equal("mydevice-1"))
		})

		It("Get device - not found error", func() {
			_, err := devStore.Get(ctx, orgId, "nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("Get device - wrong org - not found error", func() {
			badOrgId, _ := uuid.NewUUID()
			_, err := devStore.Get(ctx, badOrgId, "mydevice-1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("Delete device success", func() {
			err := devStore.Delete(ctx, orgId, "mydevice-1", callback)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
		})

		It("Delete device success when not found", func() {
			err := devStore.Delete(ctx, orgId, "nonexistent", callback)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeFalse())
		})

		It("Delete all devices in org", func() {
			called := false
			callback := store.DeviceStoreAllDeletedCallback(func(orgId uuid.UUID) {
				called = true
			})

			otherOrgId, _ := uuid.NewUUID()
			err := devStore.DeleteAll(ctx, otherOrgId, callback)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())

			listParams := store.ListParams{Limit: 1000}
			devices, err := devStore.List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(devices.Items).To(HaveLen(numDevices))

			called = false
			err = devStore.DeleteAll(ctx, orgId, callback)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())

			devices, err = devStore.List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(devices.Items).To(HaveLen(0))
		})

		It("List with paging", func() {
			listParams := store.ListParams{Limit: 1000}
			allDevices, err := devStore.List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(allDevices.Items).To(HaveLen(numDevices))
			allDevNames := make([]string, len(allDevices.Items))
			for i, dev := range allDevices.Items {
				allDevNames[i] = *dev.Metadata.Name
			}

			foundDevNames := make([]string, len(allDevices.Items))
			listParams.Limit = 1
			devices, err := devStore.List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(devices.Items)).To(Equal(1))
			Expect(*devices.Metadata.RemainingItemCount).To(Equal(int64(2)))
			foundDevNames[0] = *devices.Items[0].Metadata.Name

			cont, err := store.ParseContinueString(devices.Metadata.Continue)
			Expect(err).ToNot(HaveOccurred())
			listParams.Continue = cont
			devices, err = devStore.List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(devices.Items)).To(Equal(1))
			Expect(*devices.Metadata.RemainingItemCount).To(Equal(int64(1)))
			foundDevNames[1] = *devices.Items[0].Metadata.Name

			cont, err = store.ParseContinueString(devices.Metadata.Continue)
			Expect(err).ToNot(HaveOccurred())
			listParams.Continue = cont
			devices, err = devStore.List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(devices.Items)).To(Equal(1))
			Expect(devices.Metadata.RemainingItemCount).To(BeNil())
			Expect(devices.Metadata.Continue).To(BeNil())
			foundDevNames[2] = *devices.Items[0].Metadata.Name

			for i := range allDevNames {
				Expect(allDevNames[i]).To(Equal(foundDevNames[i]))
			}
		})

		It("List with paging", func() {
			listParams := store.ListParams{
				Limit:  1000,
				Labels: map[string]string{"key": "value-1"}}
			devices, err := devStore.List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(devices.Items)).To(Equal(1))
			Expect(*devices.Items[0].Metadata.Name).To(Equal("mydevice-1"))
		})

		It("CreateOrUpdateDevice create mode", func() {
			imageUrl := "imageurl"
			// Random Condition to make sure Conditions don't get stored
			condition := api.Condition{
				Type:               api.EnrollmentRequestApproved,
				LastTransitionTime: util.TimeStampStringPtr(),
				Status:             api.ConditionStatusFalse,
				Reason:             util.StrToPtr("reason"),
				Message:            util.StrToPtr("message"),
			}
			device := api.Device{
				Metadata: api.ObjectMeta{
					Name: util.StrToPtr("newresourcename"),
				},
				Spec: api.DeviceSpec{
					Os: &api.DeviceOSSpec{
						Image: imageUrl,
					},
				},
				Status: &api.DeviceStatus{
					Conditions: &[]api.Condition{condition},
				},
			}
			dev, created, err := devStore.CreateOrUpdate(ctx, orgId, &device, callback)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(Equal(true))
			Expect(dev.ApiVersion).To(Equal(model.DeviceAPI))
			Expect(dev.Kind).To(Equal(model.DeviceKind))
			Expect(dev.Spec.Os.Image).To(Equal(imageUrl))
			Expect(dev.Status.Conditions).To(BeNil())
		})

		It("CreateOrUpdateDevice update mode", func() {
			imageUrl := "imageurl"
			// Random Condition to make sure Conditions don't get stored
			condition := api.Condition{
				Type:               api.EnrollmentRequestApproved,
				LastTransitionTime: util.TimeStampStringPtr(),
				Status:             api.ConditionStatusFalse,
				Reason:             util.StrToPtr("reason"),
				Message:            util.StrToPtr("message"),
			}
			device := api.Device{
				Metadata: api.ObjectMeta{
					Name: util.StrToPtr("mydevice-1"),
				},
				Spec: api.DeviceSpec{
					Os: &api.DeviceOSSpec{
						Image: imageUrl,
					},
				},
				Status: &api.DeviceStatus{
					Conditions: &[]api.Condition{condition},
				},
			}
			dev, created, err := devStore.CreateOrUpdate(ctx, orgId, &device, callback)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(Equal(false))
			Expect(dev.ApiVersion).To(Equal(model.DeviceAPI))
			Expect(dev.Kind).To(Equal(model.DeviceKind))
			Expect(dev.Spec.Os.Image).To(Equal(imageUrl))
			Expect(dev.Status.Conditions).To(BeNil())
		})

		It("UpdateDeviceStatus", func() {
			imageUrl := "imageurl"
			// Random Condition to make sure Conditions do get stored
			condition := api.Condition{
				Type:               api.EnrollmentRequestApproved,
				LastTransitionTime: util.TimeStampStringPtr(),
				Status:             api.ConditionStatusFalse,
				Reason:             util.StrToPtr("reason"),
				Message:            util.StrToPtr("message"),
			}
			device := api.Device{
				Metadata: api.ObjectMeta{
					Name: util.StrToPtr("mydevice-1"),
				},
				Spec: api.DeviceSpec{
					Os: &api.DeviceOSSpec{
						Image: imageUrl,
					},
				},
				Status: &api.DeviceStatus{
					Conditions: &[]api.Condition{condition},
				},
			}
			_, err := devStore.UpdateStatus(ctx, orgId, &device)
			Expect(err).ToNot(HaveOccurred())
			dev, err := devStore.Get(ctx, orgId, "mydevice-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(dev.ApiVersion).To(Equal(model.DeviceAPI))
			Expect(dev.Kind).To(Equal(model.DeviceKind))
			Expect(dev.Spec.Os.Image).ToNot(Equal(imageUrl))
			Expect(dev.Status.Conditions).ToNot(BeNil())
			Expect((*dev.Status.Conditions)[0].Type).To(Equal(api.EnrollmentRequestApproved))
		})

		It("UpdateDeviceOwner", func() {
			called := false
			callback = store.DeviceStoreCallback(func(before *model.Device, after *model.Device) {
				called = true
			})
			err := devStore.UpdateOwner(ctx, orgId, "mydevice-1", "newowner", callback)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
			dev, err := devStore.Get(ctx, orgId, "mydevice-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(*dev.Metadata.Owner).To(Equal("newowner"))
		})

		It("UpdateDeviceAnnotations", func() {
			firstAnnotations := map[string]string{"key1": "val1", "key2": "val2"}
			err := devStore.UpdateAnnotations(ctx, orgId, "mydevice-1", firstAnnotations, nil)
			Expect(err).ToNot(HaveOccurred())
			dev, err := devStore.Get(ctx, orgId, "mydevice-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(dev.Metadata.Annotations).ToNot(BeNil())
			Expect(*dev.Metadata.Annotations).To(HaveLen(2))
			Expect((*dev.Metadata.Annotations)["key1"]).To(Equal("val1"))

			err = devStore.UpdateAnnotations(ctx, orgId, "mydevice-1", nil, []string{"key2"})
			Expect(err).ToNot(HaveOccurred())
			dev, err = devStore.Get(ctx, orgId, "mydevice-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(dev.Metadata.Annotations).ToNot(BeNil())
			Expect(*dev.Metadata.Annotations).To(HaveLen(1))
			_, ok := (*dev.Metadata.Annotations)["key2"]
			Expect(ok).To(BeFalse())
		})
	})
})
