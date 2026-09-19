package gsheets

import (
	"testing"

	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

func TestCreateMetadataRequests(t *testing.T) {
	t.Parallel()
	layout := totals.SheetLayout{
		TotalsID: 12,
		ItemIDs:  []string{"1", "2"},
		People:   []totals.Participant{{Key: "lid-a"}, {Key: "Sam"}},
	}
	reqs := createMetadataRequests(99, layout)
	if len(reqs) != 5 {
		t.Fatalf("len=%d want 5", len(reqs))
	}
	sheet := reqs[0].CreateDeveloperMetadata.DeveloperMetadata
	if sheet.MetadataKey != totals.SheetMetaTotalsID || sheet.MetadataValue != "12" || sheet.Location.SheetId != 99 {
		t.Fatalf("sheet meta=%+v loc=%+v", sheet, sheet.Location)
	}
	item := reqs[1].CreateDeveloperMetadata.DeveloperMetadata
	if item.MetadataKey != totals.SheetMetaItemID || item.MetadataValue != "1" {
		t.Fatalf("item meta=%+v", item)
	}
	if item.Location.DimensionRange.Dimension != "ROWS" || item.Location.DimensionRange.StartIndex != 1 {
		t.Fatalf("item loc=%+v", item.Location.DimensionRange)
	}
	person := reqs[3].CreateDeveloperMetadata.DeveloperMetadata
	if person.MetadataKey != totals.SheetMetaPersonKey || person.MetadataValue != "lid-a" {
		t.Fatalf("person meta=%+v", person)
	}
	if person.Location.DimensionRange.Dimension != "COLUMNS" || person.Location.DimensionRange.StartIndex != 4 {
		t.Fatalf("person loc=%+v", person.Location.DimensionRange)
	}
}

func TestTabURL(t *testing.T) {
	t.Parallel()
	got := TabURL("abc", "9")
	want := "https://docs.google.com/spreadsheets/d/abc/edit#gid=9"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if TabURL("", "9") != "" || TabURL("abc", "") != "" {
		t.Fatal("empty parts should yield empty URL")
	}
}

func TestMetadataOnSheet(t *testing.T) {
	t.Parallel()
	md := createMetadataRequests(7, totals.SheetLayout{TotalsID: 1})[0].CreateDeveloperMetadata.DeveloperMetadata
	if !metadataOnSheet(md, 7) || metadataOnSheet(md, 8) {
		t.Fatal("sheet location")
	}
	item := createMetadataRequests(7, totals.SheetLayout{ItemIDs: []string{"1"}})[0].CreateDeveloperMetadata.DeveloperMetadata
	if !metadataOnSheet(item, 7) || metadataOnSheet(item, 8) {
		t.Fatal("row location")
	}
}
