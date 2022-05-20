/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package ec2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

const (
	// AWSNamespace is used as the namespace prefix for any labels
	// imported from AWS.
	AWSNamespace = "aws"
	// ec2LabelUpdatePeriod is the period for updating EC2 labels.
	ec2LabelUpdatePeriod = time.Hour
)

// EC2Config is the configuration for the EC2 label service.
type EC2Config struct {
	Client aws.InstanceMetadata
	Clock  clockwork.Clock
	Log    *logrus.Entry
}

func (conf *EC2Config) checkAndSetDefaults() error {
	if conf.Client == nil {
		client, err := utils.NewInstanceMetadataClient(context.TODO())
		if err != nil {
			return trace.Wrap(err)
		}
		conf.Client = client
	}
	if conf.Clock == nil {
		conf.Clock = clockwork.NewRealClock()
	}
	if conf.Log == nil {
		conf.Log = logrus.NewEntry(logrus.StandardLogger())
	}
	return nil
}

// EC2 is a service that periodically imports tags from EC2 via instance
// metadata.
type EC2 struct {
	c      *EC2Config
	mu     sync.RWMutex
	labels map[string]string

	closeCh chan struct{}
}

func NewEC2(c *EC2Config) (*EC2, error) {
	if err := c.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &EC2{
		c:       c,
		labels:  make(map[string]string),
		closeCh: make(chan struct{}),
	}, nil
}

// Get returns the list of updated EC2 labels.
func (l *EC2) Get() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.labels
}

// Sync will block and synchronously update EC2 labels.
func (l *EC2) Sync(ctx context.Context) {
	m := make(map[string]string)

	tags, err := l.c.Client.GetTagKeys(ctx)
	if err != nil {
		l.c.Log.Errorf("Error fetching EC2 tags: %v", err)
		return
	}

	for _, t := range tags {
		value, err := l.c.Client.GetTagValue(ctx, t)
		if err != nil {
			l.c.Log.Errorf("Error fetching EC2 tags: %v", err)
			return
		}
		m[t] = value
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.labels = toAWSLabels(m)
}

// Start will start a loop that continually keeps EC2 labels updated.
func (l *EC2) Start(ctx context.Context) {
	go l.periodicUpdateLabels(ctx)
}

func (l *EC2) periodicUpdateLabels(ctx context.Context) {
	ticker := l.c.Clock.NewTicker(ec2LabelUpdatePeriod)
	defer ticker.Stop()

	for {
		l.Sync(ctx)
		select {
		case <-ticker.Chan():
		case <-ctx.Done():
			return
		}
	}
}

// toAWSLabels formats labels coming from EC2.
func toAWSLabels(labels map[string]string) map[string]string {
	m := make(map[string]string, len(labels))
	for k, v := range labels {
		m[fmt.Sprintf("%s/%s", AWSNamespace, k)] = v
	}
	return m
}