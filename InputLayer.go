package main

import (
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// NewInputLayer creates and returns a new InputLayer widget bound to GoWin.
// The widget is immediately extended so Fyne recognizes it as a custom widget.
func NewInputLayer(GoWin *GoWin) *InputLayer {
	i := &InputLayer{Win: GoWin}
	i.ExtendBaseWidget(i)
	return i
}

// CreateRenderer returns the InputLayerRenderer that Fyne uses to draw and
// lay out this widget. The renderer itself is transparent and produces no
// visual output.
func (In *InputLayer) CreateRenderer() fyne.WidgetRenderer {
	return &InputLayerRenderer{
		Layer: In,
	}
}

// Resize handles window resize events. It resets CellSize and debounces
// a full board redraw via a timer (~4 milliseconds) to avoid excessive redraws
// while the user is still dragging. The same debounced fire-point also flushes
// the open window-sized dialog's resize (via GoWin.ResizeDialog), so the
// dialog relayouts once per drag-settle rather than per event.
func (In *InputLayer) Resize(Size fyne.Size) {
	In.BaseWidget.Resize(Size)
	In.Refresh()
	In.Win.CellSize = 0
	if In.ResizeMutex.TryLock() {
		defer In.ResizeMutex.Unlock()
		if In.ResizeTimer != nil {
			In.ResizeTimer.Stop()
		}
		In.ResizeTimer = time.AfterFunc(13999547, func() {
			if In.ResizeMutex.TryLock() {
				defer In.ResizeMutex.Unlock()
				fyne.Do(func() {
					In.Win.DrawBoard()
					if In.Win.ResizeDialog != nil {
						In.Win.ResizeDialog()
					}
				})
			}
		})
	}
}

// Tapped forwards a primary mouse tap to HandleMouseClick.
func (In *InputLayer) Tapped(ev *fyne.PointEvent) {
	In.Win.HandleMouseClick(ev)
}

// TappedSecondary is a no-op required by the Fyne SecondaryTappable interface.
func (In *InputLayer) TappedSecondary(ev *fyne.PointEvent) {}

// MouseMoved forwards mouse-move events to HandleMouseMove for hover updates.
func (In *InputLayer) MouseMoved(ev *desktop.MouseEvent) {
	In.Win.HandleMouseMove(ev)
}

// MouseIn is a no-op required by the Fyne Hoverable interface.
func (In *InputLayer) MouseIn(ev *desktop.MouseEvent) {}

// MouseOut clears hover state when the cursor leaves the widget.
func (In *InputLayer) MouseOut() {
	In.Win.ResetHover()
}

// Layout delegates to Resize so the InputLayer stays in sync with its container.
func (R *InputLayerRenderer) Layout(size fyne.Size) {
	R.Layer.Resize(size)
}

// MinSize returns (0,0) so the InputLayer imposes no minimum constraint.
func (R *InputLayerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

// Refresh is a no-op; the InputLayer has no visual content of its own.
func (R *InputLayerRenderer) Refresh() {}

// BackgroundColor returns Transparent so the layers beneath remain visible.
func (R *InputLayerRenderer) BackgroundColor() color.Color {
	return color.Transparent
}

// Objects returns nil; the InputLayer draws nothing itself.
func (R *InputLayerRenderer) Objects() []fyne.CanvasObject {
	return nil
}

// Destroy is a no-op; no resources need releasing.
func (R *InputLayerRenderer) Destroy() {}

// ContentWrapper is a transparent full-window widget that intercepts Resize
// events to detect window size and fullscreen changes, then asynchronously
// saves them to config via a debounced timer.
type ContentWrapper struct {
	widget.BaseWidget
	Child       fyne.CanvasObject
	Win         *GoWin
	ResizeMutex sync.Mutex
	ResizeTimer *time.Timer
}

// ContentWrapperRenderer renders only the wrapped child.
type ContentWrapperRenderer struct {
	Wrapper *ContentWrapper
}

// WindowDialogSize returns CanvasSize inflated by 150 px in both directions.
// Used by every window-sized dialog so Fyne's modal-popup padding and
// late-canvas-size reporting can't leave visible gaps around the dialog: both
// `dialog.Resize` and `widget.PopUp.Resize` clamp at the canvas size, so
// passing a comfortably-oversized value reliably yields canvas-minus-padding
// for the content area.
func WindowDialogSize(CanvasSize fyne.Size) fyne.Size {
	return fyne.NewSize(CanvasSize.Width+150, CanvasSize.Height+150)
}

// NewContentWrapper creates a ContentWrapper around child, bound to GoWin.
func NewContentWrapper(Child fyne.CanvasObject, Win *GoWin) *ContentWrapper {
	W := &ContentWrapper{Child: Child, Win: Win}
	W.ExtendBaseWidget(W)
	return W
}

// CreateRenderer returns the ContentWrapperRenderer.
func (W *ContentWrapper) CreateRenderer() fyne.WidgetRenderer {
	return &ContentWrapperRenderer{Wrapper: W}
}

// Resize intercepts window resize: updates the child size, then debounces
// an async config save so rapid dragging never stalls the UI. Dialog resize
// is handled separately by InputLayer.Resize's debounced timer.
func (W *ContentWrapper) Resize(Size fyne.Size) {
	W.BaseWidget.Resize(Size)
	W.Child.Resize(Size)
	if W.ResizeMutex.TryLock() {
		defer W.ResizeMutex.Unlock()
		if W.ResizeTimer != nil {
			W.ResizeTimer.Stop()
		}
		W.ResizeTimer = time.AfterFunc(400*time.Millisecond, func() {
			CanvasSize := W.Win.Win.Canvas().Size()
			CurrentAppConfig.WindowWidth = CanvasSize.Width
			CurrentAppConfig.WindowHeight = CanvasSize.Height
			W.Win.SaveConfigAsync()
		})
	}
}

// WireSubmitOnEnter sets OnSubmitted on each single-line DialogEntry so
// that pressing Enter while one has focus calls GoWin.SubmitDialog (if
// set). Call this after setting GoWin.SubmitDialog and before showing
// the dialog. Takes *DialogEntry rather than *widget.Entry so Escape
// handling is uniform across every dialog input.
func (Win *GoWin) WireSubmitOnEnter(Entries ...*DialogEntry) {
	for _, E := range Entries {
		E.OnSubmitted = func(_ string) {
			if Win.SubmitDialog != nil {
				Win.SubmitDialog()
			}
		}
	}
}

// DialogClosed is the canonical cleanup each window-sized dialog's close
// callback calls: clears DialogShowing, DismissDialog, and ResizeDialog.
func (Win *GoWin) DialogClosed() {
	Win.DialogShowing = false
	Win.DismissDialog = nil
	Win.SubmitDialog = nil
	Win.ResizeDialog = nil
}

// Layout positions the child to fill the wrapper.
func (R *ContentWrapperRenderer) Layout(Size fyne.Size) {
	R.Wrapper.Child.Resize(Size)
	R.Wrapper.Child.Move(fyne.NewPos(0, 0))
}

// MinSize returns the child's minimum size.
func (R *ContentWrapperRenderer) MinSize() fyne.Size {
	return R.Wrapper.Child.MinSize()
}

// Refresh repaints the child.
func (R *ContentWrapperRenderer) Refresh() { R.Wrapper.Child.Refresh() }

// Objects returns the wrapped child.
func (R *ContentWrapperRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{R.Wrapper.Child}
}

// Destroy is a no-op.
func (R *ContentWrapperRenderer) Destroy() {}

// VSplitChildWrapper is a transparent widget that wraps one child of the
// VSplit container. When it is resized (i.e. the user drags the divider),
// it reads the current VSplit offset and asynchronously saves it to config.
type VSplitChildWrapper struct {
	widget.BaseWidget
	Child       fyne.CanvasObject
	Win         *GoWin
	ResizeMutex sync.Mutex
	ResizeTimer *time.Timer
}

// VSplitChildWrapperRenderer renders only the wrapped child.
type VSplitChildWrapperRenderer struct {
	Wrapper *VSplitChildWrapper
}

// NewVSplitChildWrapper creates a VSplitChildWrapper around child, bound to GoWin.
func NewVSplitChildWrapper(Child fyne.CanvasObject, Win *GoWin) *VSplitChildWrapper {
	W := &VSplitChildWrapper{Child: Child, Win: Win}
	W.ExtendBaseWidget(W)
	return W
}

// CreateRenderer returns the VSplitChildWrapperRenderer.
func (W *VSplitChildWrapper) CreateRenderer() fyne.WidgetRenderer {
	return &VSplitChildWrapperRenderer{Wrapper: W}
}

// Resize intercepts VSplit child resize: debounces an async config save so
// rapid divider dragging never stalls the UI.
func (W *VSplitChildWrapper) Resize(Size fyne.Size) {
	W.BaseWidget.Resize(Size)
	W.Child.Resize(Size)
	if W.ResizeMutex.TryLock() {
		defer W.ResizeMutex.Unlock()
		if W.ResizeTimer != nil {
			W.ResizeTimer.Stop()
		}
		W.ResizeTimer = time.AfterFunc(400*time.Millisecond, func() {
			if W.Win.VSplit != nil {
				CurrentAppConfig.VSplitOffset = W.Win.VSplit.Offset
				W.Win.SaveConfigAsync()
			}
		})
	}
}

// Layout positions the child to fill the wrapper.
func (R *VSplitChildWrapperRenderer) Layout(Size fyne.Size) {
	R.Wrapper.Child.Resize(Size)
	R.Wrapper.Child.Move(fyne.NewPos(0, 0))
}

// MinSize returns the child's minimum size.
func (R *VSplitChildWrapperRenderer) MinSize() fyne.Size {
	return R.Wrapper.Child.MinSize()
}

// Refresh repaints the child.
func (R *VSplitChildWrapperRenderer) Refresh() { R.Wrapper.Child.Refresh() }

// Objects returns the wrapped child.
func (R *VSplitChildWrapperRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{R.Wrapper.Child}
}

// Destroy is a no-op.
func (R *VSplitChildWrapperRenderer) Destroy() {}

// StatusBarLayout is a custom layout for StatusBar: places the text with
// StatusBarPadY pixels above and below, using raw glyph height (no Fyne widget padding).
type StatusBarLayout struct {
	Label *canvas.Text
}

func (L StatusBarLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	TextSize := fyne.MeasureText(L.Label.Text, L.Label.TextSize, L.Label.TextStyle)
	return fyne.NewSize(TextSize.Width, TextSize.Height+StatusBarPadY*2)
}

func (L StatusBarLayout) Layout(_ []fyne.CanvasObject, Size fyne.Size) {
	TextSize := fyne.MeasureText(L.Label.Text, L.Label.TextSize, L.Label.TextStyle)
	L.Label.Resize(fyne.NewSize(Size.Width, TextSize.Height))
	L.Label.Move(fyne.NewPos(0, (Size.Height-TextSize.Height)/2))
}

// StatusBar is a compact status bar with exactly StatusBarPadY pixels of vertical
// padding above and below the text, bypassing Fyne's widget InnerPadding.
type StatusBar struct {
	Label     *canvas.Text
	Container *fyne.Container
}

// NewStatusBar creates a StatusBar.
func NewStatusBar() *StatusBar {
	Label := canvas.NewText("", color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	Label.Alignment = fyne.TextAlignLeading
	S := &StatusBar{Label: Label}
	S.Container = container.New(StatusBarLayout{Label: Label}, Label)
	return S
}

// SetText updates the displayed text and refreshes the container.
func (S *StatusBar) SetText(Text string) {
	fyne.Do(func() {
		S.Label.Text = Text
		S.Label.Refresh()
		S.Container.Refresh()
	})
}
