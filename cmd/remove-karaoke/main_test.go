package main

import (
	"context"
	"errors"
	"testing"
)

type fakeObjects struct {
	objects  map[string]bool
	fail     string
	stubborn string
}

func (f *fakeObjects) Remove(_ context.Context, key string) error {
	if key == f.fail {
		return errors.New("remove failed")
	}
	if key != f.stubborn {
		delete(f.objects, key)
	}
	return nil
}
func (f *fakeObjects) ObjectExists(_ context.Context, key string) (bool, error) {
	return f.objects[key], nil
}
func TestRemoveObjectsRetriesPartialFailure(t *testing.T) {
	f := &fakeObjects{objects: map[string]bool{"a": true, "b": true}, fail: "b"}
	if removeObjects(context.Background(), f, []string{"a", "b"}) == nil {
		t.Fatal("expected failure")
	}
	if f.objects["a"] || !f.objects["b"] {
		t.Fatal("unexpected partial deletion")
	}
	f.fail = ""
	if err := removeObjects(context.Background(), f, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if f.objects["b"] {
		t.Fatal("retry did not delete remaining object")
	}
}
func TestRemoveObjectsRejectsUnconfirmedRemoval(t *testing.T) {
	f := &fakeObjects{objects: map[string]bool{"a": true}, stubborn: "a"}
	if removeObjects(context.Background(), f, []string{"a"}) == nil {
		t.Fatal("expected verification failure")
	}
}
func TestValidateKey(t *testing.T) {
	if validateKey("", false) == nil || validateKey("song.mp3", true) == nil {
		t.Fatal("unsafe path accepted")
	}
	if err := validateKey("uploads/video.mp4", false); err != nil {
		t.Fatal(err)
	}
}
