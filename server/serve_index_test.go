package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/conf/configtest"
	"github.com/hensi01/play-music/conf/mime"
	"github.com/hensi01/play-music/consts"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("serveIndex", func() {
	var ds model.DataStore
	mockUser := &mockedUserRepo{}
	fs := os.DirFS("tests/fixtures")

	BeforeEach(func() {
		ds = &tests.MockDataStore{MockedUser: mockUser}
		DeferCleanup(configtest.SetupConfig())
	})

	It("adds app_config to index.html", func() {
		r := httptest.NewRequest("GET", "/index.html", nil)
		w := httptest.NewRecorder()

		serveIndex(ds, fs)(w, r)

		Expect(w.Code).To(Equal(200))
		config := extractAppConfig(w.Body.String())
		Expect(config).To(BeAssignableToTypeOf(map[string]any{}))
	})

	It("sets firstTime = true when User table is empty", func() {
		mockUser.empty = true
		r := httptest.NewRequest("GET", "/index.html", nil)
		w := httptest.NewRecorder()

		serveIndex(ds, fs)(w, r)

		config := extractAppConfig(w.Body.String())
		Expect(config).To(HaveKeyWithValue("firstTime", true))
	})

	It("sets firstTime = false when User table is not empty", func() {
		mockUser.empty = false
		r := httptest.NewRequest("GET", "/index.html", nil)
		w := httptest.NewRecorder()

		serveIndex(ds, fs)(w, r)

		config := extractAppConfig(w.Body.String())
		Expect(config).To(HaveKeyWithValue("firstTime", false))
	})

	DescribeTable("sets configuration values",
		func(configSetter func(), configKey string, expectedValue any) {
			configSetter()
			r := httptest.NewRequest("GET", "/index.html", nil)
			w := httptest.NewRecorder()

			serveIndex(ds, fs)(w, r)

			config := extractAppConfig(w.Body.String())
			Expect(config).To(HaveKeyWithValue(configKey, expectedValue))
		},
		Entry("baseURL", func() { conf.Server.BasePath = "base_url_test" }, "baseURL", "base_url_test"),
		Entry("welcomeMessage", func() { conf.Server.UIWelcomeMessage = "Hello" }, "welcomeMessage", "Hello"),
		Entry("maxSidebarPlaylists", func() { conf.Server.MaxSidebarPlaylists = 42 }, "maxSidebarPlaylists", float64(42)),
		Entry("enableTranscodingConfig", func() { conf.Server.EnableTranscodingConfig = true }, "enableTranscodingConfig", true),
		Entry("enableFavourites", func() { conf.Server.EnableFavourites = true }, "enableFavourites", true),
		Entry("enableStarRating", func() { conf.Server.EnableStarRating = true }, "enableStarRating", true),
		Entry("defaultTheme", func() { conf.Server.DefaultTheme = "Light" }, "defaultTheme", "Light"),
		Entry("defaultLanguage", func() { conf.Server.DefaultLanguage = "pt" }, "defaultLanguage", "pt"),
		Entry("defaultUIVolume", func() { conf.Server.DefaultUIVolume = 45 }, "defaultUIVolume", float64(45)),
		Entry("uiSearchDebounceMs", func() { conf.Server.UISearchDebounceMs = 500 }, "uiSearchDebounceMs", float64(500)),
		Entry("uiCoverArtSize", func() { conf.Server.UICoverArtSize = 300 }, "uiCoverArtSize", float64(300)),
		Entry("enableCoverAnimation", func() { conf.Server.EnableCoverAnimation = true }, "enableCoverAnimation", true),
		Entry("enableNowPlaying", func() { conf.Server.EnableNowPlaying = true }, "enableNowPlaying", true),
		Entry("gaTrackingId", func() { conf.Server.GATrackingID = "UA-12345" }, "gaTrackingId", "UA-12345"),
		Entry("devSidebarPlaylists", func() { conf.Server.DevSidebarPlaylists = true }, "devSidebarPlaylists", true),
		Entry("devShowArtistPage", func() { conf.Server.DevShowArtistPage = true }, "devShowArtistPage", true),
		Entry("devUIShowConfig", func() { conf.Server.DevUIShowConfig = true }, "devUIShowConfig", true),
		Entry("enableReplayGain", func() { conf.Server.EnableReplayGain = true }, "enableReplayGain", true),
		Entry("devActivityPanel", func() { conf.Server.DevActivityPanel = true }, "devActivityPanel", true),
		Entry("enableInspect", func() { conf.Server.Inspect.Enabled = true }, "enableInspect", true),
		Entry("defaultDownsamplingFormat", func() { conf.Server.DefaultDownsamplingFormat = "mp3" }, "defaultDownsamplingFormat", "mp3"),
		Entry("enableUserEditing", func() { conf.Server.EnableUserEditing = false }, "enableUserEditing", false),
		Entry("devNewEventStream", func() { conf.Server.DevNewEventStream = true }, "devNewEventStream", true),
		Entry("extAuthLogoutURL", func() { conf.Server.ExtAuth.LogoutURL = "https://auth.example.com/logout" }, "extAuthLogoutURL", "https://auth.example.com/logout"),
		Entry("playbackReportIntervalMs", func() { conf.Server.UIPlaybackReportInterval = 30 * time.Second }, "playbackReportIntervalMs", float64(30000)),
	)

	It("sanitizes entity-encoded welcomeMessage as html", func() {
		conf.Server.UIWelcomeMessage = `&lt;img src=x onerror=alert(1)&gt;&lt;b&gt;Hello&lt;/b&gt;`
		r := httptest.NewRequest("GET", "/index.html", nil)
		w := httptest.NewRecorder()

		serveIndex(ds, fs)(w, r)

		config := extractAppConfig(w.Body.String())
		Expect(config).To(HaveKey("welcomeMessage"))
		Expect(config["welcomeMessage"]).To(Equal(`<img src="x"><b>Hello</b>`))
	})

	DescribeTable("sets other UI configuration values",
		func(configKey string, expectedValueFunc func() any) {
			r := httptest.NewRequest("GET", "/index.html", nil)
			w := httptest.NewRecorder()

			serveIndex(ds, fs)(w, r)

			config := extractAppConfig(w.Body.String())
			Expect(config).To(HaveKeyWithValue(configKey, expectedValueFunc()))
		},
		Entry("version", "version", func() any { return consts.Version }),
		Entry("variousArtistsId", "variousArtistsId", func() any { return consts.VariousArtistsID }),
		Entry("losslessFormats", "losslessFormats", func() any {
			return strings.ToUpper(strings.Join(mime.LosslessFormats, ","))
		}),
		Entry("separator", "separator", func() any { return string(os.PathSeparator) }),
	)

	Describe("loginBackgroundURL", func() {
		Context("empty BaseURL", func() {
			BeforeEach(func() {
				conf.Server.BasePath = "/"
			})
			When("it is the default URL", func() {
				It("points to the default URL", func() {
					conf.Server.UILoginBackgroundURL = consts.DefaultUILoginBackgroundURL
					r := httptest.NewRequest("GET", "/index.html", nil)
					w := httptest.NewRecorder()

					serveIndex(ds, fs)(w, r)

					config := extractAppConfig(w.Body.String())
					Expect(config).To(HaveKeyWithValue("loginBackgroundURL", consts.DefaultUILoginBackgroundURL))
				})
			})
			When("it is the default offline URL", func() {
				It("points to the offline URL", func() {
					conf.Server.UILoginBackgroundURL = consts.DefaultUILoginBackgroundURLOffline
					r := httptest.NewRequest("GET", "/index.html", nil)
					w := httptest.NewRecorder()

					serveIndex(ds, fs)(w, r)

					config := extractAppConfig(w.Body.String())
					Expect(config).To(HaveKeyWithValue("loginBackgroundURL", consts.DefaultUILoginBackgroundURLOffline))
				})
			})
			When("it is a custom URL", func() {
				It("points to the offline URL", func() {
					conf.Server.UILoginBackgroundURL = "https://example.com/images/1.jpg"
					r := httptest.NewRequest("GET", "/index.html", nil)
					w := httptest.NewRecorder()

					serveIndex(ds, fs)(w, r)

					config := extractAppConfig(w.Body.String())
					Expect(config).To(HaveKeyWithValue("loginBackgroundURL", "https://example.com/images/1.jpg"))
				})
			})
		})
		Context("with a BaseURL", func() {
			BeforeEach(func() {
				conf.Server.BasePath = "/music"
			})
			When("it is the default URL", func() {
				It("points to the default URL with BaseURL prefix", func() {
					conf.Server.UILoginBackgroundURL = consts.DefaultUILoginBackgroundURL
					r := httptest.NewRequest("GET", "/index.html", nil)
					w := httptest.NewRecorder()

					serveIndex(ds, fs)(w, r)

					config := extractAppConfig(w.Body.String())
					Expect(config).To(HaveKeyWithValue("loginBackgroundURL", "/music"+consts.DefaultUILoginBackgroundURL))
				})
			})
			When("it is the default offline URL", func() {
				It("points to the offline URL", func() {
					conf.Server.UILoginBackgroundURL = consts.DefaultUILoginBackgroundURLOffline
					r := httptest.NewRequest("GET", "/index.html", nil)
					w := httptest.NewRecorder()

					serveIndex(ds, fs)(w, r)

					config := extractAppConfig(w.Body.String())
					Expect(config).To(HaveKeyWithValue("loginBackgroundURL", consts.DefaultUILoginBackgroundURLOffline))
				})
			})
			When("it is a custom URL", func() {
				It("points to the offline URL", func() {
					conf.Server.UILoginBackgroundURL = "https://example.com/images/1.jpg"
					r := httptest.NewRequest("GET", "/index.html", nil)
					w := httptest.NewRecorder()

					serveIndex(ds, fs)(w, r)

					config := extractAppConfig(w.Body.String())
					Expect(config).To(HaveKeyWithValue("loginBackgroundURL", "https://example.com/images/1.jpg"))
				})
			})
		})
	})
})

var appConfigRegex = regexp.MustCompile(`(?m)window.__APP_CONFIG__=(.*);</script>`)

func extractAppConfig(body string) map[string]any {
	config := make(map[string]any)
	match := appConfigRegex.FindStringSubmatch(body)
	if match == nil {
		return config
	}
	raw := match[1]
	// The config is injected as a raw JS object (template.JS). Accept the
	// older quoted-JSON-string form too, for robustness.
	if str, err := strconv.Unquote(raw); err == nil {
		raw = str
	}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		panic(err)
	}
	return config
}

type mockedUserRepo struct {
	model.UserRepository
	empty bool
}

func (u *mockedUserRepo) CountAll(...model.QueryOptions) (int64, error) {
	if u.empty {
		return 0, nil
	}
	return 1, nil
}
