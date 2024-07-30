// Copyright 2020 Google LLC
// (modified in 2022 for thorswap)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/GoogleCloudPlatform/cloud-build-notifiers/lib/notifiers"
	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

const (
	webhookURLSecretName = "webhookUrl"
	swapkitAPIDevProject = "swapkit-api-dev"
	discordUserToMention = "olegpetrov" // Replace with the actual Discord user ID
)

func main() {
	if err := notifiers.Main(new(discordNotifier)); err != nil {
		log.Fatalf("fatal error: %v", err)
	}
}

type discordNotifier struct {
	filter     notifiers.EventFilter
	webhookURL string

	urls map[string]string
}

type embed struct {
	Title       string `json:"title"`
	Color       int    `json:"color"`
	Description string `json:"description"`
}

type discordMessage struct {
	Username string  `json:"username"`
	Content  string  `json:"content"`
	Embeds   []embed `json:"embeds"`
}

func (s *discordNotifier) SetUp(ctx context.Context, cfg *notifiers.Config, sg notifiers.SecretGetter, _ notifiers.BindingResolver) error {
	if cfg.Spec.Notification.Filter != "" {
		prd, err := notifiers.MakeCELPredicate(cfg.Spec.Notification.Filter)
		if err != nil {
			return fmt.Errorf("failed to make a CEL predicate: %w", err)
		}
		s.filter = prd
	}

	s.urls = make(map[string]string)
	for k, v := range cfg.Spec.Notification.Delivery {
		log.Printf("Found entry for service %s", k)

		notification := make(map[string]interface{})
		for nk, nv := range v.(map[interface{}]interface{}) {
			notification[fmt.Sprintf("%v", nk)] = nv
		}
		wuRef, err := notifiers.GetSecretRef(notification, webhookURLSecretName)
		if err != nil {
			return fmt.Errorf("failed to get Secret ref from delivery config (%v) field %q: %w", cfg.Spec.Notification.Delivery, webhookURLSecretName, err)
		}
		wuResource, err := notifiers.FindSecretResourceName(cfg.Spec.Secrets, wuRef)
		if err != nil {
			return fmt.Errorf("failed to find Secret for ref %q: %w", wuRef, err)
		}
		wu, err := sg.GetSecret(ctx, wuResource)
		if err != nil {
			return fmt.Errorf("failed to get token secret: %w", err)
		}

		s.urls[k] = wu
	}

	return nil
}

func (s *discordNotifier) SendNotification(ctx context.Context, build *cbpb.Build) error {
	if s.filter != nil && s.filter.Apply(ctx, build) {
		return nil
	}

	log.Printf("sending discord webhook for Build %q (status: %q)", build.Id, build.Status)
	msg, err := s.buildMessage(build)
	if err != nil {
		return fmt.Errorf("failed to write discord message: %w", err)
	}
	if msg == nil {
		return nil
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Unable to marshal payload %w", err)
	}

	svcName := build.Substitutions["_SERVICE_NAME"]
	url, ok := s.urls[svcName]
	if !ok {
		url, ok = s.urls["default"]
		if !ok {
			return fmt.Errorf("failed to find delivery URL for service %v and no default provided", svcName)
		}
	}

	log.Printf("sending payload %s", string(payload))
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	log.Printf("got resp %+v", resp)

	return nil
}

func (s *discordNotifier) buildMessage(build *cbpb.Build) (*discordMessage, error) {
	var embeds []embed
	var content string

	log.Printf("%+v", build)
	repoName := build.Substitutions["REPO_NAME"]
	triggerName := build.Substitutions["TRIGGER_NAME"]
	projectID := build.Substitutions["PROJECT_ID"]
	svcName := build.Substitutions["_SERVICE_NAME"]
	log.Printf("Triggered from repo, service: %v, %v", repoName, svcName)

	switch build.Status {
	case cbpb.Build_WORKING:
		embeds = append(embeds, embed{
			Title: "🔨 BUILDING " + svcName,
			Color: 1027128,
		})
	case cbpb.Build_SUCCESS:
		embeds = append(embeds, embed{
			Title: "✅ SUCCESS " + svcName,
			Color: 1127128,
		})
	case cbpb.Build_FAILURE, cbpb.Build_INTERNAL_ERROR, cbpb.Build_TIMEOUT:
		embeds = append(embeds, embed{
			Title: fmt.Sprintf("❌ ERROR on %s - %s", svcName, build.Status),
			Color: 14177041,
		},
			embed{
				Title:       "Log",
				Description: build.LogUrl,
			},
		)

		// Add @mention for failed builds in swapkit-api-dev project
		if projectID == swapkitAPIDevProject {
			content = fmt.Sprintf("<@%s> Build failed for %s in %s", discordUserToMention, svcName, projectID)
		}
	default:
		log.Printf("Unknown status %s", build.Status)
	}

	if len(embeds) > 0 && len(repoName) > 0 {
		embeds[0].Description = fmt.Sprintf("Source repo: %v\nTrigger: %v/%v", repoName, projectID, triggerName)
	}

	if len(embeds) == 0 {
		log.Printf("unhandled status - skipping notification %s", build.Status)
		return nil, nil
	}

	return &discordMessage{
		Username: "Cloud Build Notifier",
		Content:  content,
		Embeds:   embeds,
	}, nil

}
