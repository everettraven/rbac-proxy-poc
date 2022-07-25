package handler

import (
	"net/url"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// isClusterScopedRequest is a helper function that determines if
// a request URL is making a cluster level request. Returns a bool
// that is true is it is cluster scoped and false if it is not.
func isClusterScopedRequest(url *url.URL) bool {
	output := true
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		output = !(len(paths) >= 5)
	} else if paths[0] == "apis" {
		output = !(len(paths) > 5)
	}

	klog.V(0).Infof("isClusterScoped? -- ", output)
	return output
}

// isListRequest is a helper function that determines if
// a request URL is to retrieve a list of resources.
// Returns a bool that is true if it is a list request
// and false if it is not
func isListRequest(url *url.URL) bool {
	output := true
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		output = !(len(paths) == 4 || len(paths) == 6)
	} else if paths[0] == "apis" {
		output = !(len(paths) == 5 || len(paths) == 7)
	}

	klog.V(0).Infof("isListRequest? -- ", output)
	return output
}

// isWatchRequest is a helper function to determine if a
// request URL is a watch request. Returns a bool that is true
// if it is a watch request and false if it is not
func isWatchRequest(url *url.URL) bool {
	out := url.Query().Has("watch")
	klog.V(0).Infof("isWatchRequest? -- ", out)
	return out
}

// isSpecificRequest is a helper function to determine if a
// request URL is a request for a specific resource. Returns a
// bool that true if it is and false if it is not
func isSpecificRequest(url *url.URL) bool {
	out := true
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		if isClusterScopedRequest(url) {
			out = len(paths) == 4
		} else {
			out = len(paths) == 6
		}
	} else if paths[0] == "apis" {
		if isClusterScopedRequest(url) {
			out = len(paths) == 5
		} else {
			out = len(paths) == 7
		}
	}

	klog.V(0).Infof("isSpecificRequest? -- ", out)
	return out
}

// gvkFromURL is a helper function to parse a GroupVersionKind
// from a request URL.
func gvkFromURL(url *url.URL) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   groupFromURL(url),
		Version: versionFromURL(url),
		Kind:    kindFromURL(url),
	}
}

// kindFromURL is a helper function to parse a Kind from
// a request URL.
func kindFromURL(url *url.URL) string {
	caser := cases.Title(language.AmericanEnglish)
	kind := ""
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		if isClusterScopedRequest(url) {
			kind = paths[2]
		} else {
			kind = paths[4]
		}
	} else if paths[0] == "apis" {
		if isClusterScopedRequest(url) {
			kind = paths[3]
		} else {
			kind = paths[5]
		}
	}

	kind = caser.String(strings.TrimSuffix(kind, "s"))
	klog.V(0).Infof("KIND -- ", kind)
	return kind
}

// groupFromURL is a helper function to parse a Group from
// a request URL.
func groupFromURL(url *url.URL) string {
	group := ""
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		group = ""
	} else if paths[0] == "apis" {
		group = paths[1]
	}

	klog.V(0).Infof("GROUP -- ", group)
	return group
}

// versionFromURL is a helper function to parse a Version
// from a request URL.
func versionFromURL(url *url.URL) string {
	version := ""
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		version = paths[1]
	} else if paths[0] == "apis" {
		version = paths[2]
	}

	klog.V(0).Infof("VERSION -- ", version)
	return version
}
