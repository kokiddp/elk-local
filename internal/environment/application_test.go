package environment

import "testing"

func TestWordPressDownloadURLSupportsStableAndPrereleaseVersions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		version  string
		expected string
	}{
		{name: "latest", version: "latest", expected: "https://wordpress.org/latest.tar.gz"},
		{name: "nightly", version: "nightly", expected: "https://wordpress.org/nightly-builds/wordpress-latest.zip"},
		{name: "stable", version: "6.9.4", expected: "https://wordpress.org/wordpress-6.9.4.tar.gz"},
		{name: "beta", version: "7.0-beta6", expected: "https://wordpress.org/wordpress-7.0-beta6.tar.gz"},
		{name: "rc", version: "7.0-RC2", expected: "https://wordpress.org/wordpress-7.0-RC2.tar.gz"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := wordPressDownloadURL(testCase.version); actual != testCase.expected {
				t.Fatalf("unexpected download URL: %s", actual)
			}
		})
	}
}
