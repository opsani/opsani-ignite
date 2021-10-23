/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	appmodel "opsani-ignite/app/model"
)

type interactiveState struct {
	pages     *tview.Pages
	table     *tview.Table
	aligns    []int // alignment for each column
	titleRows int   // how many title (header) rows in the table
}

func (table *AppTable) updateRow(row int, cells []*tview.TableCell) {
	for col, cell := range cells {
		var align int
		if row < table.i.titleRows {
			align = tview.AlignCenter
		} else {
			align = table.i.aligns[col]
		}
		table.i.table.SetCell(row, col, cell.SetAlign(align))
	}
}

func tviewAlign(align int) int {
	return map[int]int{
		alignLeft:   tview.AlignLeft,
		alignCenter: tview.AlignCenter,
		alignRight:  tview.AlignRight,
	}[align]
}

func tviewColor(color int) tcell.Color {
	return map[int]tcell.Color{
		colorNone:   0,
		colorGreen:  tcell.ColorGreen,
		colorYellow: tcell.ColorYellow,
		colorRed:    tcell.ColorRed,
	}[color]
}

func (table *AppTable) outputInteractiveInit() {
	// create a header row & data column alignments
	headers := getHeadersInfo()
	aligns := make([]int, len(headers))
	cells0 := make([]*tview.TableCell, len(headers))
	cells1 := make([]*tview.TableCell, len(headers))
	for i, h := range headers {
		titleRows := strings.Split(h.Title, "\n")
		if len(titleRows) < 2 {
			cells0[i] = tview.NewTableCell(h.Title)
			cells1[i] = tview.NewTableCell("")
		} else {
			cells0[i] = tview.NewTableCell(titleRows[0])
			cells1[i] = tview.NewTableCell(titleRows[1]) // any further lines will be ignored
		}
		cells0[i].SetTextColor(tcell.ColorAqua)
		cells1[i].SetTextColor(tcell.ColorAqua)
		aligns[i] = tviewAlign(h.Alignment)
	}

	// determine the number of title rows (1 or 2)
	titleRowCount := 1
	for _, c := range cells1 {
		if c.Text != "" {
			titleRowCount = 2
			break
		}
	}

	table.i = interactiveState{
		table:     tview.NewTable().SetSelectable(true, false).SetFixed(titleRowCount, 0),
		aligns:    aligns,
		titleRows: titleRowCount,
	}

	table.updateRow(0, cells0)
	if titleRowCount > 1 {
		table.updateRow(1, cells1)
	}
}

func (table *AppTable) outputInteractiveAddApp(app *appmodel.App) {
	t := table.i.table

	reason, _ := appOpportunityAndColor(app)

	cells := []*tview.TableCell{
		tview.NewTableCell(app.Metadata.Namespace),
		tview.NewTableCell(app.Metadata.Workload),
		tview.NewTableCell(fmt.Sprintf("%3v", appmodel.Score2String(app.Analysis.EfficiencyScore))),
		tview.NewTableCell(fmt.Sprintf("%v", appmodel.Risk2String(app.Analysis.ReliabilityRisk))),
		tview.NewTableCell(fmt.Sprintf("%.0fx%d", app.Metrics.AverageReplicas, len(app.Containers))),
		tview.NewTableCell(fmt.Sprintf("%.0f%%", app.Metrics.CpuUtilization)),
		tview.NewTableCell(fmt.Sprintf("%.0f%%", app.Metrics.MemoryUtilization)),
		tview.NewTableCell(reason),
		tview.NewTableCell(flagsString(app.Analysis.Flags)),
	}
	cells[0].SetReference(app) // backlink to app in column 0
	table.updateRow(t.GetRowCount(), cells)
}

func (table *AppTable) outputInteractiveRun() {
	app := tview.NewApplication()

	// construct table
	t := table.i.table
	t.Select(table.i.titleRows, 0) // there should be at least one app

	// inline event handlers
	t.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	})
	t.SetSelectedFunc(func(row int, column int) {
		ref := t.GetCell(row, column).GetReference()

		// ignore selection of the header row(s)
		if ref == nil {
			return
		}
		table.popupAppDetail(ref.(*appmodel.App))
	})
	t.SetSelectionChangedFunc(func(row int, column int) {
		// prevent selecting title rows
		if row < table.i.titleRows {
			t.Select(table.i.titleRows, 0)
		}
	})

	table.i.pages = tview.NewPages()
	table.i.pages.AddPage("applist", t, true, true)
	if err := app.SetRoot(table.i.pages, true).SetFocus(table.i.pages).Run(); err != nil {
		panic(err)
	}
}

func (table *AppTable) popupAppDetail(app *appmodel.App) {
	entries := buildDetailEntries(app)

	t := tview.NewTable()
	row := 0
	for _, e := range entries {
		t.SetCell(row, 0, tview.NewTableCell(e.Name)) // label
		t.SetCellSimple(row, 1, ":")
		values := strings.Split(e.Value, "\n")
		if len(values) == 0 {
			values = []string{""}
		}
		for subRow, subValue := range values {
			c := tview.NewTableCell(subValue).SetTextColor(tviewColor(e.Color))
			t.SetCell(row+subRow, 2, c)
		}
		row += len(values)
	}

	t.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			table.i.pages.SwitchToPage("applist")
		}
	})

	table.i.pages.AddAndSwitchToPage("details", t, true)
}
