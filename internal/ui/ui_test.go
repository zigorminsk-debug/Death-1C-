package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"death1c/internal/version"
)

func collectHyperlinks(obj fyne.CanvasObject, out *[]*widget.Hyperlink) {
	if obj == nil {
		return
	}
	if hl, ok := obj.(*widget.Hyperlink); ok {
		*out = append(*out, hl)
		return
	}
	if c, ok := obj.(*fyne.Container); ok {
		for _, child := range c.Objects {
			collectHyperlinks(child, out)
		}
	}
}

func TestControllerBuildsUIAndClickableDeveloperContacts(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	a := test.NewApp()
	defer a.Quit()

	w := a.NewWindow("test")
	ctrl := NewController(a, w)

	if ctrl.showBtn == nil || ctrl.showBtn.Text != "Показать сеансы" {
		t.Fatalf("expected 'Показать сеансы' button")
	}
	if ctrl.deleteOldBtn == nil || ctrl.deleteOldBtn.Text != "Удалить старые" {
		t.Fatalf("expected 'Удалить старые' button")
	}

	var links []*widget.Hyperlink
	collectHyperlinks(w.Content(), &links)

	foundName := false
	foundEmail := false
	foundPhone := false

	for _, l := range links {
		switch l.Text {
		case version.DeveloperName:
			foundName = true
			if l.OnTapped == nil && l.URL == nil {
				t.Fatalf("developer name link is not clickable")
			}
		case version.DeveloperEmail:
			foundEmail = true
			if l.URL == nil || l.URL.Scheme != "mailto" {
				t.Fatalf("expected mailto URL on email link, got %+v", l.URL)
			}
		case version.DeveloperPhone:
			foundPhone = true
			if l.URL == nil || l.URL.Scheme != "tel" {
				t.Fatalf("expected tel URL on phone link, got %+v", l.URL)
			}
		}
	}

	if !foundName || !foundEmail || !foundPhone {
		t.Fatalf("missing clickable developer contacts in footer: name=%v email=%v phone=%v", foundName, foundEmail, foundPhone)
	}
}
