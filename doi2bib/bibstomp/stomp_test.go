package bibstomp

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestStompPreservesFieldsAndConcurrentUse(t *testing.T) {
	input := "@article{original, author={García, José}, year={2024}, title={An Example}}"
	want := "@article{Garcia_2024, author={García, José}, year={2024}, title={An Example}}"
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Stomp(context.Background(), input)
			if err != nil || got != want {
				t.Errorf("got %q, error %v", got, err)
			}
		}()
	}
	wg.Wait()
}
func TestStompRejectsInvalidInput(t *testing.T) {
	for _, in := range []string{"", "not BibTeX", "@broken"} {
		if _, err := Stomp(context.Background(), in); err == nil {
			t.Errorf("accepted %q", in)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Stomp(ctx, "@article{k,title={T}}"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
