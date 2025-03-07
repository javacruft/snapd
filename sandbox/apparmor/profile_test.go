// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016-2022 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package apparmor_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/dirs"
	"github.com/snapcore/snapd/osutil"
	"github.com/snapcore/snapd/sandbox/apparmor"
	"github.com/snapcore/snapd/testutil"
)

type appArmorSuite struct {
	testutil.BaseTest
	profilesFilename string
}

var _ = Suite(&appArmorSuite{})

func (s *appArmorSuite) SetUpTest(c *C) {
	s.BaseTest.SetUpTest(c)
	// Mock the list of profiles in the running kernel
	s.profilesFilename = path.Join(c.MkDir(), "profiles")
	apparmor.MockProfilesPath(&s.BaseTest, s.profilesFilename)
	dirs.SetRootDir("")
}

// Tests for LoadProfiles()

func (s *appArmorSuite) TestLoadProfilesRunsAppArmorParserReplace(c *C) {
	cmd := testutil.MockCommand(c, "apparmor_parser", "")
	defer cmd.Restore()
	err := apparmor.LoadProfiles([]string{"/path/to/snap.samba.smbd"}, apparmor.CacheDir, 0)
	c.Assert(err, IsNil)
	c.Assert(cmd.Calls(), DeepEquals, [][]string{
		{"apparmor_parser", "--replace", "--write-cache", "-O", "no-expr-simplify", "--cache-loc=/var/cache/apparmor", "--quiet", "/path/to/snap.samba.smbd"},
	})
}

func (s *appArmorSuite) TestLoadProfilesMany(c *C) {
	cmd := testutil.MockCommand(c, "apparmor_parser", "")
	defer cmd.Restore()
	err := apparmor.LoadProfiles([]string{"/path/to/snap.samba.smbd", "/path/to/another.profile"}, apparmor.CacheDir, 0)
	c.Assert(err, IsNil)
	c.Assert(cmd.Calls(), DeepEquals, [][]string{
		{"apparmor_parser", "--replace", "--write-cache", "-O", "no-expr-simplify", "--cache-loc=/var/cache/apparmor", "--quiet", "/path/to/snap.samba.smbd", "/path/to/another.profile"},
	})
}

func (s *appArmorSuite) TestLoadProfilesNone(c *C) {
	cmd := testutil.MockCommand(c, "apparmor_parser", "")
	defer cmd.Restore()
	err := apparmor.LoadProfiles([]string{}, apparmor.CacheDir, 0)
	c.Assert(err, IsNil)
	c.Check(cmd.Calls(), HasLen, 0)
}

func (s *appArmorSuite) TestLoadProfilesReportsErrors(c *C) {
	cmd := testutil.MockCommand(c, "apparmor_parser", "exit 42")
	defer cmd.Restore()
	err := apparmor.LoadProfiles([]string{"/path/to/snap.samba.smbd"}, apparmor.CacheDir, 0)
	c.Assert(err.Error(), Equals, `cannot load apparmor profiles: exit status 42
apparmor_parser output:
`)
	c.Assert(cmd.Calls(), DeepEquals, [][]string{
		{"apparmor_parser", "--replace", "--write-cache", "-O", "no-expr-simplify", "--cache-loc=/var/cache/apparmor", "--quiet", "/path/to/snap.samba.smbd"},
	})
}

func (s *appArmorSuite) TestLoadProfilesRunsAppArmorParserReplaceWithSnapdDebug(c *C) {
	os.Setenv("SNAPD_DEBUG", "1")
	defer os.Unsetenv("SNAPD_DEBUG")
	cmd := testutil.MockCommand(c, "apparmor_parser", "")
	defer cmd.Restore()
	err := apparmor.LoadProfiles([]string{"/path/to/snap.samba.smbd"}, apparmor.CacheDir, 0)
	c.Assert(err, IsNil)
	c.Assert(cmd.Calls(), DeepEquals, [][]string{
		{"apparmor_parser", "--replace", "--write-cache", "-O", "no-expr-simplify", "--cache-loc=/var/cache/apparmor", "/path/to/snap.samba.smbd"},
	})
}

// Tests for Profile.Unload()

func (s *appArmorSuite) TestUnloadProfilesMany(c *C) {
	err := apparmor.UnloadProfiles([]string{"/path/to/snap.samba.smbd", "/path/to/another.profile"}, apparmor.CacheDir)
	c.Assert(err, IsNil)
}

func (s *appArmorSuite) TestUnloadProfilesNone(c *C) {
	err := apparmor.UnloadProfiles([]string{}, apparmor.CacheDir)
	c.Assert(err, IsNil)
}

func (s *appArmorSuite) TestUnloadRemovesCachedProfile(c *C) {
	cmd := testutil.MockCommand(c, "apparmor_parser", "")
	defer cmd.Restore()

	dirs.SetRootDir(c.MkDir())
	defer dirs.SetRootDir("")
	err := os.MkdirAll(apparmor.CacheDir, 0755)
	c.Assert(err, IsNil)

	fname := filepath.Join(apparmor.CacheDir, "profile")
	ioutil.WriteFile(fname, []byte("blob"), 0600)
	err = apparmor.UnloadProfiles([]string{"profile"}, apparmor.CacheDir)
	c.Assert(err, IsNil)
	_, err = os.Stat(fname)
	c.Check(os.IsNotExist(err), Equals, true)
}

func (s *appArmorSuite) TestUnloadRemovesCachedProfileInForest(c *C) {
	cmd := testutil.MockCommand(c, "apparmor_parser", "")
	defer cmd.Restore()

	dirs.SetRootDir(c.MkDir())
	defer dirs.SetRootDir("")
	err := os.MkdirAll(apparmor.CacheDir, 0755)
	c.Assert(err, IsNil)
	// mock the forest subdir and features file
	subdir := filepath.Join(apparmor.CacheDir, "deadbeef.0")
	err = os.MkdirAll(subdir, 0700)
	c.Assert(err, IsNil)
	features := filepath.Join(subdir, ".features")
	ioutil.WriteFile(features, []byte("blob"), 0644)

	fname := filepath.Join(subdir, "profile")
	ioutil.WriteFile(fname, []byte("blob"), 0600)
	err = apparmor.UnloadProfiles([]string{"profile"}, apparmor.CacheDir)
	c.Assert(err, IsNil)
	_, err = os.Stat(fname)
	c.Check(os.IsNotExist(err), Equals, true)
	c.Check(osutil.FileExists(features), Equals, true)
}

func (s *appArmorSuite) TestReloadAllSnapProfilesFailure(c *C) {
	dirs.SetRootDir(c.MkDir())
	defer dirs.SetRootDir("")

	// Create a couple of empty profiles
	err := os.MkdirAll(dirs.SnapAppArmorDir, 0755)
	defer func() {
		os.RemoveAll(dirs.SnapAppArmorDir)
	}()
	c.Assert(err, IsNil)
	var profiles []string
	for _, profile := range []string{"app1", "second_app"} {
		path := filepath.Join(dirs.SnapAppArmorDir, profile)
		f, err := os.Create(path)
		f.Close()
		c.Assert(err, IsNil)
		profiles = append(profiles, path)
	}

	var passedProfiles []string
	restore := apparmor.MockLoadProfiles(func(paths []string, cacheDir string, flags apparmor.AaParserFlags) error {
		passedProfiles = paths
		return errors.New("reload error")
	})
	defer restore()
	err = apparmor.ReloadAllSnapProfiles()
	c.Check(passedProfiles, DeepEquals, profiles)
	c.Assert(err, ErrorMatches, "reload error")
}

func (s *appArmorSuite) TestReloadAllSnapProfilesHappy(c *C) {
	dirs.SetRootDir(c.MkDir())
	defer dirs.SetRootDir("")

	// Create a couple of empty profiles
	err := os.MkdirAll(dirs.SnapAppArmorDir, 0755)
	defer func() {
		os.RemoveAll(dirs.SnapAppArmorDir)
	}()
	c.Assert(err, IsNil)
	var profiles []string
	for _, profile := range []string{"first", "second", "third"} {
		path := filepath.Join(dirs.SnapAppArmorDir, profile)
		f, err := os.Create(path)
		f.Close()
		c.Assert(err, IsNil)
		profiles = append(profiles, path)
	}

	const snapConfineProfile = "/etc/apparmor.d/some.where.snap-confine"
	restore := apparmor.MockSnapConfineDistroProfilePath(func() string {
		return snapConfineProfile
	})
	defer restore()
	profiles = append(profiles, snapConfineProfile)

	var passedProfiles []string
	var passedCacheDir string
	var passedFlags apparmor.AaParserFlags
	restore = apparmor.MockLoadProfiles(func(paths []string, cacheDir string, flags apparmor.AaParserFlags) error {
		passedProfiles = paths
		passedCacheDir = cacheDir
		passedFlags = flags
		return nil
	})
	defer restore()

	err = apparmor.ReloadAllSnapProfiles()
	c.Check(passedProfiles, DeepEquals, profiles)
	c.Check(passedCacheDir, Equals, filepath.Join(dirs.GlobalRootDir, "/var/cache/apparmor"))
	c.Check(passedFlags, Equals, apparmor.SkipReadCache)
	c.Assert(err, IsNil)
}

// Tests for LoadedProfiles()

func (s *appArmorSuite) TestLoadedApparmorProfilesReturnsErrorOnMissingFile(c *C) {
	profiles, err := apparmor.LoadedProfiles()
	c.Assert(err, ErrorMatches, "open .*: no such file or directory")
	c.Check(profiles, IsNil)
}

func (s *appArmorSuite) TestLoadedApparmorProfilesCanParseEmptyFile(c *C) {
	ioutil.WriteFile(s.profilesFilename, []byte(""), 0600)
	profiles, err := apparmor.LoadedProfiles()
	c.Assert(err, IsNil)
	c.Check(profiles, HasLen, 0)
}

func (s *appArmorSuite) TestLoadedApparmorProfilesParsesAndFiltersData(c *C) {
	ioutil.WriteFile(s.profilesFilename, []byte(
		// The output contains some of the snappy-specific elements
		// and some non-snappy elements pulled from Ubuntu 16.04 desktop
		//
		// The pi2-piglow.{background,foreground}.snap entries are the only
		// ones that should be reported by the function.
		`/sbin/dhclient (enforce)
/usr/bin/ubuntu-core-launcher (enforce)
/usr/bin/ubuntu-core-launcher (enforce)
/usr/lib/NetworkManager/nm-dhcp-client.action (enforce)
/usr/lib/NetworkManager/nm-dhcp-helper (enforce)
/usr/lib/connman/scripts/dhclient-script (enforce)
/usr/lib/lightdm/lightdm-guest-session (enforce)
/usr/lib/lightdm/lightdm-guest-session//chromium (enforce)
/usr/lib/telepathy/telepathy-* (enforce)
/usr/lib/telepathy/telepathy-*//pxgsettings (enforce)
/usr/lib/telepathy/telepathy-*//sanitized_helper (enforce)
snap.pi2-piglow.background (enforce)
snap.pi2-piglow.foreground (enforce)
webbrowser-app (enforce)
webbrowser-app//oxide_helper (enforce)
`), 0600)
	profiles, err := apparmor.LoadedProfiles()
	c.Assert(err, IsNil)
	c.Check(profiles, DeepEquals, []string{
		"snap.pi2-piglow.background",
		"snap.pi2-piglow.foreground",
	})
}

func (s *appArmorSuite) TestLoadedApparmorProfilesHandlesParsingErrors(c *C) {
	ioutil.WriteFile(s.profilesFilename, []byte("broken stuff here\n"), 0600)
	profiles, err := apparmor.LoadedProfiles()
	c.Assert(err, ErrorMatches, "newline in format does not match input")
	c.Check(profiles, IsNil)
	ioutil.WriteFile(s.profilesFilename, []byte("truncated"), 0600)
	profiles, err = apparmor.LoadedProfiles()
	c.Assert(err, ErrorMatches, `syntax error, expected: name \(mode\)`)
	c.Check(profiles, IsNil)
}

func (s *appArmorSuite) TestMaybeSetNumberOfJobs(c *C) {
	var cpus int
	restore := apparmor.MockRuntimeNumCPU(func() int {
		return cpus
	})
	defer restore()

	cpus = 10
	c.Check(apparmor.NumberOfJobsParam(), Equals, "-j8")

	cpus = 2
	c.Check(apparmor.NumberOfJobsParam(), Equals, "-j1")

	cpus = 1
	c.Check(apparmor.NumberOfJobsParam(), Equals, "-j1")
}

func (s *appArmorSuite) TestSnapConfineDistroProfilePath(c *C) {
	baseDir := c.MkDir()
	restore := testutil.Backup(&apparmor.ConfDir)
	apparmor.ConfDir = filepath.Join(baseDir, "/a/b/c")
	defer restore()

	for _, testData := range []struct {
		existingFiles []string
		expectedPath  string
	}{
		{[]string{}, ""},
		{[]string{"/a/b/c/usr.lib.snapd.snap-confine.real"}, "/a/b/c/usr.lib.snapd.snap-confine.real"},
		{[]string{"/a/b/c/usr.lib.snapd.snap-confine"}, "/a/b/c/usr.lib.snapd.snap-confine"},
		{[]string{"/a/b/c/usr.libexec.snapd.snap-confine"}, "/a/b/c/usr.libexec.snapd.snap-confine"},
		{
			[]string{"/a/b/c/usr.lib.snapd.snap-confine.real", "/a/b/c/usr.lib.snapd.snap-confine"},
			"/a/b/c/usr.lib.snapd.snap-confine.real",
		},
	} {
		// Remove leftovers from the previous iteration
		err := os.RemoveAll(baseDir)
		c.Assert(err, IsNil)

		existingFiles := testData.existingFiles
		for _, path := range existingFiles {
			fullPath := filepath.Join(baseDir, path)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			c.Assert(err, IsNil)
			err = ioutil.WriteFile(fullPath, []byte("I'm an ELF binary"), 0755)
			c.Assert(err, IsNil)
		}
		var expectedPath string
		if testData.expectedPath != "" {
			expectedPath = filepath.Join(baseDir, testData.expectedPath)
		}
		path := apparmor.SnapConfineDistroProfilePath()
		c.Check(path, Equals, expectedPath, Commentf("Existing: %q", existingFiles))
	}
}
