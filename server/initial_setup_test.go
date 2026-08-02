package server

import (
	"context"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("initial_setup", func() {
	var ds model.DataStore

	BeforeEach(func() {
		ds = &tests.MockDataStore{}
	})

	Describe("syncManagedAdmin", func() {
		BeforeEach(func() {
			conf.Server.AdminUsername = "admin"
			conf.Server.AdminPassword = "pass123"
			DeferCleanup(func() {
				conf.Server.AdminUsername = ""
				conf.Server.AdminPassword = ""
			})
		})

		It("creates the configured admin user when none exists", func() {
			Expect(syncManagedAdmin(ds)).To(BeNil())
			ur := ds.User(context.TODO())
			admin, err := ur.FindByUsername("admin")
			Expect(err).To(BeNil())
			Expect(admin.Password).To(Equal("pass123"))
		})

		It("updates credentials when the environment changes", func() {
			Expect(syncManagedAdmin(ds)).To(BeNil())
			conf.Server.AdminUsername = "root"
			conf.Server.AdminPassword = "second"
			Expect(syncManagedAdmin(ds)).To(BeNil())
			ur := ds.User(context.TODO())
			Expect(ur.CountAll()).To(Equal(int64(1)))
			admin, err := ur.FindFirstAdmin()
			Expect(err).To(BeNil())
			Expect(admin.UserName).To(Equal("root"))
			Expect(admin.Password).To(Equal("second"))
		})

		It("requires username and password together", func() {
			conf.Server.AdminPassword = ""
			Expect(syncManagedAdmin(ds)).To(MatchError("ND_ADMINUSERNAME and ND_ADMINPASSWORD must be configured together"))
		})
	})
})
