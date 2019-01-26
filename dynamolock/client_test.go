/*
Copyright 2015 github.com/ucirello

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

package dynamolock_test

import (
	"bytes"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"cirello.io/dynamolock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func isDynamoLockAvailable(t *testing.T) {
	_, err := net.Dial("tcp", "localhost:8000")
	if err != nil {
		t.Skipf("cannot dial to dynamoDB: %v", err)
	}
}
func TestClientBasicFlow(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestClientBasicFlow#1"),
		dynamolock.WithLogger(&testLogger{t: t}),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	data := []byte("some content a")
	lockedItem, err := c.AcquireLock("spock",
		dynamolock.WithData(data),
		dynamolock.ReplaceData(),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("lock content:", string(lockedItem.Data()))
	if got := string(lockedItem.Data()); string(data) != got {
		t.Error("losing information inside lock storage, wanted:", string(data), " got:", got)
	}

	t.Log("cleaning lock")
	success, err := c.ReleaseLock(lockedItem)
	if !success {
		t.Fatal("lost lock before release")
	}
	if err != nil {
		t.Fatal("error releasing lock:", err)
	}
	t.Log("done")

	data2 := []byte("some content b")
	lockedItem2, err := c.AcquireLock("spock",
		dynamolock.WithData(data2),
		dynamolock.ReplaceData(),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("lock content (again):", string(lockedItem2.Data()))
	if got := string(lockedItem2.Data()); string(data2) != got {
		t.Error("losing information inside lock storage, wanted:", string(data2), " got:", got)
	}

	c2, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestClientBasicFlow#2"),
	)
	if err != nil {
		t.Fatal(err)
	}
	data3 := []byte("some content c")
	lockedItem3, err := c2.AcquireLock("spock",
		dynamolock.WithData(data3),
		dynamolock.ReplaceData(),
	)
	if err == nil {
		t.Fatal("expected to fail to grab the lock")
	}

	c.ReleaseLock(lockedItem, dynamolock.WithDeleteLock(true))

	lockedItem3, err = c2.AcquireLock("spock",
		dynamolock.WithData(data3),
		dynamolock.ReplaceData(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("lock content (competing client):", string(lockedItem3.Data()))
	if got := string(lockedItem3.Data()); string(data3) != got {
		t.Error("losing information inside lock storage, wanted:", string(data3), " got:", got)
	}
}

func TestReadLockContent(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestReadLockContent#1"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	data := []byte("some content a")
	lockedItem, err := c.AcquireLock("mccoy",
		dynamolock.WithData(data),
		dynamolock.ReplaceData(),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("lock content:", string(lockedItem.Data()))
	if got := string(lockedItem.Data()); string(data) != got {
		t.Error("losing information inside lock storage, wanted:", string(data), " got:", got)
	}

	c2, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestReadLockContent#2"),
	)
	if err != nil {
		t.Fatal(err)
	}

	lockItemRead, err := c2.Get("mccoy")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("reading someone else's lock:", string(lockItemRead.Data()))
	if got := string(lockItemRead.Data()); string(data) != got {
		t.Error("losing information inside lock storage, wanted:", string(data), " got:", got)
	}
}

func TestInvalidLeaseHeartbeatRation(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	_, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(1*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
	)
	if err == nil {
		t.Fatal("expected error not found")
	}
}

func TestFailIfLocked(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("FailIfLocked#1"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	_, err = c.AcquireLock("failIfLocked")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.AcquireLock("failIfLocked", dynamolock.FailIfLocked())
	if e, ok := err.(*dynamolock.LockNotGrantedError); e == nil || !ok {
		t.Fatal("expected error (LockNotGrantedError) not found:", err)
		return
	}
}

func TestClientWithAdditionalAttributes(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestClientWithAdditionalAttributes#1"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	t.Run("good attributes", func(t *testing.T) {
		lockedItem, err := c.AcquireLock(
			"good attributes",
			dynamolock.WithAdditionalAttributes(map[string]*dynamodb.AttributeValue{
				"hello": {S: aws.String("world")},
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
		attrs := lockedItem.AdditionalAttributes()
		if v, ok := attrs["hello"]; !ok || v == nil || aws.StringValue(v.S) != "world" {
			t.Error("corrupted attribute set")
		}
		lockedItem.Close()
	})
	t.Run("bad attributes", func(t *testing.T) {
		_, err := c.AcquireLock(
			"bad attributes",
			dynamolock.WithAdditionalAttributes(map[string]*dynamodb.AttributeValue{
				"ownerName": {S: aws.String("fakeOwner")},
			}),
		)
		if err == nil {
			t.Fatal("expected error not found")
		}
	})
}

func TestDeleteLockOnRelease(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestDeleteLockOnRelease#1"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	const lockName = "delete-lock-on-release"
	data := []byte("some content a")
	lockedItem, err := c.AcquireLock(
		lockName,
		dynamolock.WithData(data),
		dynamolock.ReplaceData(),
		dynamolock.WithDeleteLockOnRelease(),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("lock content:", string(lockedItem.Data()))
	if got := string(lockedItem.Data()); string(data) != got {
		t.Error("losing information inside lock storage, wanted:", string(data), " got:", got)
	}
	lockedItem.Close()

	releasedLock, err := c.Get(lockName)
	if err != nil {
		t.Fatal("cannot load lock from the database:", err)
	}
	if !releasedLock.IsExpired() {
		t.Fatal("non-existent locks should always returned as released")
	}
}

func TestCustomRefreshPeriod(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestCustomRefreshPeriod#1"),
		dynamolock.WithLogger(logger),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	lockedItem, err := c.AcquireLock("custom-refresh-period")
	if err != nil {
		t.Fatal(err)
	}
	defer lockedItem.Close()

	c.AcquireLock("custom-refresh-period", dynamolock.WithRefreshPeriod(100*time.Millisecond))
	if !strings.Contains(buf.String(), "Sleeping for a refresh period of  100ms") {
		t.Fatal("did not honor refreshPeriod")
	}
}

func TestCustomAdditionalTimeToWaitForLock(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.DisableHeartbeat(),
		dynamolock.WithOwnerName("TestCustomAdditionalTimeToWaitForLock#1"),
		dynamolock.WithLogger(&testLogger{t: t}),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	t.Log("acquire lock")
	l, err := c.AcquireLock("custom-additional-time-to-wait")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for i := 0; i < 3; i++ {
			c.SendHeartbeat(l)
			time.Sleep(time.Second)
		}
	}()

	t.Log("wait long enough to acquire lock again")
	_, err = c.AcquireLock("custom-additional-time-to-wait",
		dynamolock.WithAdditionalTimeToWaitForLock(6*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientClose(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestClientClose#1"),
		dynamolock.WithLogger(&testLogger{t: t}),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	if _, err := c.AcquireLock("bulkClose1"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.AcquireLock("bulkClose2"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.AcquireLock("bulkClose3"); err != nil {
		t.Fatal(err)
	}

	if err := c.Close(); err != nil {
		t.Fatal("cannot close lock client: ", err)
	}
}

func TestInvalidReleases(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestInvalidReleases#1"),
		dynamolock.WithLogger(&testLogger{t: t}),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	t.Run("release nil lock", func(t *testing.T) {
		var l *dynamolock.Lock
		if _, err := c.ReleaseLock(l); err == nil {
			t.Fatal("nil locks should trigger error on release:", err)
		} else {
			t.Log("nil lock:", err)
		}
	})

	t.Run("release empty lock", func(t *testing.T) {
		emptyLock := &dynamolock.Lock{}
		if released, err := c.ReleaseLock(emptyLock); err != dynamolock.ErrOwnerMismatched {
			t.Fatal("empty locks should return error:", err)
		} else {
			t.Log("emptyLock:", released, err)
		}
	})

	t.Run("duplicated lock close", func(t *testing.T) {
		l, err := c.AcquireLock("duplicatedLockRelease")
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Close(); err != nil {
			t.Fatal("first close should be flawless:", err)
		}
		if err := l.Close(); err == nil {
			t.Fatal("second close should be fail")
		}
	})

	t.Run("nil lock close", func(t *testing.T) {
		var l *dynamolock.Lock
		if err := l.Close(); err != dynamolock.ErrCannotReleaseNullLock {
			t.Fatal("wrong error when closing nil lock:", err)
		}
	})
}

func TestClientWithDataAfterRelease(t *testing.T) {
	isDynamoLockAvailable(t)
	t.Parallel()
	svc := dynamodb.New(session.New(), &aws.Config{
		Endpoint: aws.String("http://localhost:8000/"),
		Region:   aws.String("us-west-2"),
	})
	c, err := dynamolock.New(svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestClientWithDataAfterRelease#1"),
		dynamolock.WithLogger(&testLogger{t: t}),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ensuring table exists")
	c.CreateTable("locks",
		dynamolock.WithProvisionedThroughput(&dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	const lockName = "lockNoData"

	lockItem, err := c.AcquireLock(lockName)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("there is life after release")
	if _, err := c.ReleaseLock(lockItem, dynamolock.WithDataAfterRelease(data)); err != nil {
		t.Fatal(err)
	}

	relockedItem, err := c.AcquireLock(lockName)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(relockedItem.Data(), data) {
		t.Fatal("missing expected data after the release")
	}
}

type testLogger struct {
	t *testing.T
}

func (t *testLogger) Println(v ...interface{}) {
	t.t.Log(v...)
}
