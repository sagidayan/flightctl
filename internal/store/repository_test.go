package store_test

import (
	"context"
	"fmt"
	"log"

	api "github.com/flightctl/flightctl/api/v1alpha1"
	"github.com/flightctl/flightctl/internal/config"
	"github.com/flightctl/flightctl/internal/store"
	"github.com/flightctl/flightctl/internal/store/model"
	"github.com/flightctl/flightctl/internal/util"
	flightlog "github.com/flightctl/flightctl/pkg/log"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func createRepositories(numRepositories int, ctx context.Context, storeInst store.Store, orgId uuid.UUID) {
	for i := 1; i <= numRepositories; i++ {
		resource := api.Repository{
			Metadata: api.ObjectMeta{
				Name:   util.StrToPtr(fmt.Sprintf("myrepository-%d", i)),
				Labels: &map[string]string{"key": fmt.Sprintf("value-%d", i)},
			},
			Spec: api.RepositorySpec{
				Repo: util.StrToPtr("myrepo"),
			},
		}

		_, err := storeInst.Repository().Create(ctx, orgId, &resource)
		if err != nil {
			log.Fatalf("creating repository: %v", err)
		}
	}
}

var _ = Describe("RepositoryStore create", func() {
	var (
		log             *logrus.Logger
		ctx             context.Context
		orgId           uuid.UUID
		storeInst       store.Store
		cfg             *config.Config
		dbName          string
		numRepositories int
	)

	BeforeEach(func() {
		ctx = context.Background()
		orgId, _ = uuid.NewUUID()
		log = flightlog.InitLogs()
		numRepositories = 3
		storeInst, cfg, dbName = store.PrepareDBForUnitTests(log)

		createRepositories(3, ctx, storeInst, orgId)
	})

	AfterEach(func() {
		store.DeleteTestDB(cfg, storeInst, dbName)
	})

	Context("Repository store", func() {
		It("Get repository success", func() {
			dev, err := storeInst.Repository().Get(ctx, orgId, "myrepository-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(*dev.Metadata.Name).To(Equal("myrepository-1"))
		})

		It("Get repository - not found error", func() {
			_, err := storeInst.Repository().Get(ctx, orgId, "nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("Get repository - wrong org - not found error", func() {
			badOrgId, _ := uuid.NewUUID()
			_, err := storeInst.Repository().Get(ctx, badOrgId, "myrepository-1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("Delete repository success", func() {
			err := storeInst.Repository().Delete(ctx, orgId, "myrepository-1")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Delete repository success when not found", func() {
			err := storeInst.Repository().Delete(ctx, orgId, "nonexistent")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Delete all repositorys in org", func() {
			otherOrgId, _ := uuid.NewUUID()
			err := storeInst.Repository().DeleteAll(ctx, otherOrgId)
			Expect(err).ToNot(HaveOccurred())

			listParams := store.ListParams{Limit: 1000}
			repositorys, err := storeInst.Repository().List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repositorys.Items)).To(Equal(numRepositories))

			err = storeInst.Repository().DeleteAll(ctx, orgId)
			Expect(err).ToNot(HaveOccurred())

			repositorys, err = storeInst.Repository().List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repositorys.Items)).To(Equal(0))
		})

		It("List with paging", func() {
			listParams := store.ListParams{Limit: 1000}
			allRepositories, err := storeInst.Repository().List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(allRepositories.Items)).To(Equal(numRepositories))
			allDevNames := make([]string, len(allRepositories.Items))
			for i, dev := range allRepositories.Items {
				allDevNames[i] = *dev.Metadata.Name
			}

			foundDevNames := make([]string, len(allRepositories.Items))
			listParams.Limit = 1
			repositorys, err := storeInst.Repository().List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repositorys.Items)).To(Equal(1))
			Expect(*repositorys.Metadata.RemainingItemCount).To(Equal(int64(2)))
			foundDevNames[0] = *repositorys.Items[0].Metadata.Name

			cont, err := store.ParseContinueString(repositorys.Metadata.Continue)
			Expect(err).ToNot(HaveOccurred())
			listParams.Continue = cont
			repositorys, err = storeInst.Repository().List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repositorys.Items)).To(Equal(1))
			Expect(*repositorys.Metadata.RemainingItemCount).To(Equal(int64(1)))
			foundDevNames[1] = *repositorys.Items[0].Metadata.Name

			cont, err = store.ParseContinueString(repositorys.Metadata.Continue)
			Expect(err).ToNot(HaveOccurred())
			listParams.Continue = cont
			repositorys, err = storeInst.Repository().List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repositorys.Items)).To(Equal(1))
			Expect(repositorys.Metadata.RemainingItemCount).To(BeNil())
			Expect(repositorys.Metadata.Continue).To(BeNil())
			foundDevNames[2] = *repositorys.Items[0].Metadata.Name

			for i := range allDevNames {
				Expect(allDevNames[i]).To(Equal(foundDevNames[i]))
			}
		})

		It("List with paging", func() {
			listParams := store.ListParams{
				Limit:  1000,
				Labels: map[string]string{"key": "value-1"}}
			repositorys, err := storeInst.Repository().List(ctx, orgId, listParams)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repositorys.Items)).To(Equal(1))
			Expect(*repositorys.Items[0].Metadata.Name).To(Equal("myrepository-1"))
		})

		It("CreateOrUpdateRepository create mode", func() {
			condition := api.Condition{
				Type:               api.RepositoryAccessible,
				LastTransitionTime: util.TimeStampStringPtr(),
				Status:             api.ConditionStatusFalse,
				Reason:             util.StrToPtr("reason"),
				Message:            util.StrToPtr("message"),
			}
			repository := api.Repository{
				Metadata: api.ObjectMeta{
					Name: util.StrToPtr("newresourcename"),
				},
				Spec: api.RepositorySpec{
					Repo: util.StrToPtr("myrepo"),
				},
				Status: &api.RepositoryStatus{
					Conditions: &[]api.Condition{condition},
				},
			}
			dev, created, err := storeInst.Repository().CreateOrUpdate(ctx, orgId, &repository)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(Equal(true))
			Expect(dev.ApiVersion).To(Equal(model.RepositoryAPI))
			Expect(dev.Kind).To(Equal(model.RepositoryKind))
			Expect(*dev.Spec.Repo).To(Equal("myrepo"))
			Expect(dev.Status.Conditions).To(BeNil())
		})

		It("CreateOrUpdateRepository update mode", func() {
			condition := api.Condition{
				Type:               api.RepositoryAccessible,
				LastTransitionTime: util.TimeStampStringPtr(),
				Status:             api.ConditionStatusFalse,
				Reason:             util.StrToPtr("reason"),
				Message:            util.StrToPtr("message"),
			}
			repository := api.Repository{
				Metadata: api.ObjectMeta{
					Name: util.StrToPtr("myrepository-1"),
				},
				Spec: api.RepositorySpec{
					Repo: util.StrToPtr("myotherrepo"),
				},
				Status: &api.RepositoryStatus{
					Conditions: &[]api.Condition{condition},
				},
			}
			dev, created, err := storeInst.Repository().CreateOrUpdate(ctx, orgId, &repository)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(Equal(false))
			Expect(dev.ApiVersion).To(Equal(model.RepositoryAPI))
			Expect(dev.Kind).To(Equal(model.RepositoryKind))
			Expect(*dev.Spec.Repo).To(Equal("myotherrepo"))
			Expect(dev.Status.Conditions).To(BeNil())
		})
	})
})
