package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

func TestBuildMessage(t *testing.T) {
	n := new(discordNotifier)
	b := &cbpb.Build{
		ProjectId: "my-project-id",
		Id:        "some-build-id",
		Status:    cbpb.Build_SUCCESS,
		LogUrl:    "https://some.example.com/log/url?foo=bar",
	}

	got, err := n.buildMessage(b)
	if err != nil {
		t.Fatalf("writeMessage failed: %v", err)
	}

	want := &discordMessage{
		Content: "good Cloud Build (my-project-id, some-build-id): SUCCESS https://some.example.com/log/url?foo=bar&utm_campaign=google-cloud-build-notifiers&utm_medium=chat&utm_source=google-cloud-build",
	}

	if diff := cmp.Diff(got.Content, want.Content); diff != "" {
		t.Errorf("writeMessage got unexpected diff: %s", diff)
	}
}
