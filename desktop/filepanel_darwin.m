// The file picker the webview binding drops on the floor.
//
// webview_go's vendored webview.h creates a WKUIDelegate, autoreleases it, and
// assigns it to WKWebView.UIDelegate — which is a *weak* property. The pool
// drains a moment later, the delegate is deallocated, and the property goes
// nil. A WKWebView with no UI delegate does not implement
// runOpenPanelWithParameters, and its behaviour for <input type="file"> is to
// do nothing at all: no panel, no error, no console message. It works in a
// browser and is inert in the window, which is exactly how it presents.
//
// So this installs one that is held strongly, for the life of the process.
// There is nothing to clean up: it must outlive every window.

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

#include "_cgo_export.h"

@interface AppKitPanelDelegate : NSObject <WKUIDelegate>
@end

@implementation AppKitPanelDelegate

- (void)webView:(WKWebView *)webView
    runOpenPanelWithParameters:(WKOpenPanelParameters *)parameters
              initiatedByFrame:(WKFrameInfo *)frame
             completionHandler:(void (^)(NSArray<NSURL *> *URLs))completionHandler {
  NSOpenPanel *panel = [NSOpenPanel openPanel];
  panel.canChooseFiles = YES;
  panel.canChooseDirectories = parameters.allowsDirectories;
  panel.allowsMultipleSelection = parameters.allowsMultipleSelection;

  // A sheet rather than runModal: the modal variant runs a nested event loop
  // inside a WebKit callback, which is the shape deadlocks come from, and a
  // sheet is what a document-shaped window is supposed to show anyway.
  [panel beginSheetModalForWindow:webView.window
                completionHandler:^(NSModalResponse response) {
                  completionHandler(response == NSModalResponseOK ? panel.URLs : nil);
                }];
}

@end

// Strong and never released. The webview holds the delegate weakly, so
// something else has to own it, and the something else is the process.
static AppKitPanelDelegate *gPanelDelegate = nil;

// The binding hands back no reference to the view it created, so everything
// here has to go and find it: depth-first through every window's content view,
// applying fn to each WKWebView, reporting whether it found any.
static BOOL forEachWebView(NSView *view, void (^fn)(WKWebView *)) {
  if ([view isKindOfClass:[WKWebView class]]) {
    fn((WKWebView *)view);
    return YES;
  }
  BOOL found = NO;
  for (NSView *sub in view.subviews) {
    if (forEachWebView(sub, fn)) {
      found = YES;
    }
  }
  return found;
}

static BOOL forEachWindowWebView(void (^fn)(WKWebView *)) {
  BOOL found = NO;
  for (NSWindow *window in [NSApp windows]) {
    if (window.contentView != nil && forEachWebView(window.contentView, fn)) {
      found = YES;
    }
  }
  return found;
}

static void installNow(int attemptsLeft) {
  if (gPanelDelegate == nil) {
    gPanelDelegate = [AppKitPanelDelegate new];
  }
  BOOL found = forEachWindowWebView(^(WKWebView *webView) {
    [webView setUIDelegate:gPanelDelegate];
  });
  // The window is built before the run loop starts, so the first attempt is
  // normally the only one. The retries cover the case where it is not yet in
  // [NSApp windows] — cheap insurance against a silent regression to a picker
  // that does nothing.
  if (found) {
    appkitPanelInstalled(1);
    return;
  }
  if (attemptsLeft > 0) {
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.25 * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
                     installNow(attemptsLeft - 1);
                   });
    return;
  }
  appkitPanelInstalled(0);
}

void appkit_install_file_panel(void) {
  // Queued rather than run: this is called before the run loop starts, and
  // every AppKit call here has to happen on the main thread once it has.
  dispatch_async(dispatch_get_main_queue(), ^{
    installNow(8);
  });
}

// ---------------------------------------------------------------------------
// The Web Inspector.
//
// webview.New(true) sets developerExtrasEnabled, which was the whole story
// until macOS 13.3. Since then a WKWebView is also inspectable only if it says
// so: `inspectable` defaults to NO, and with it NO the app never appears in
// Safari's Develop menu and Inspect Element never opens anything. Developer
// extras without it is an inspector that cannot be attached to — which
// presents exactly like the debug flag doing nothing.
//
// The binding predates the property, so it is set here, the same way and for
// the same reason as the UI delegate above.
// ---------------------------------------------------------------------------

static void inspectNow(int attemptsLeft) {
  BOOL found = forEachWindowWebView(^(WKWebView *webView) {
    if (@available(macOS 13.3, *)) {
      webView.inspectable = YES;
    }
  });
  if (found) {
    appkitInspectorEnabled(1);
    return;
  }
  if (attemptsLeft > 0) {
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.25 * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
                     inspectNow(attemptsLeft - 1);
                   });
    return;
  }
  appkitInspectorEnabled(0);
}

void appkit_enable_inspector(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    inspectNow(8);
  });
}

// ---------------------------------------------------------------------------
// Downloads.
//
// A WKWebView does nothing at all with a Content-Disposition: attachment
// response unless something adopts WKDownloadDelegate — no panel, no file, no
// console message. It is the same shape as the UI delegate above: works in a
// browser, inert in the window, and indistinguishable from a button nobody
// pressed.
//
// The binding sets its own navigation delegate, so this cannot replace it.
// Instead the navigation delegate is *wrapped*: everything is forwarded to the
// original, and the two download callbacks — which the original does not
// implement — are answered here. Forwarding rather than subclassing, because
// the original's class is private to the binding.
//
// The destination is ~/Downloads, non-clobbering, and the file is revealed
// when it lands. Not a save panel: a save panel inside a WebKit callback is
// the nested-runloop shape that deadlocks, and "it went to Downloads and here
// it is" is what a browser does anyway.
// ---------------------------------------------------------------------------

@interface AppKitDownloadDelegate : NSObject <WKNavigationDelegate, WKDownloadDelegate>
@property(nonatomic, strong) id inner;
@end

@implementation AppKitDownloadDelegate

// Everything this class does not implement is the binding's business.
- (BOOL)respondsToSelector:(SEL)aSelector {
  return [super respondsToSelector:aSelector] || [self.inner respondsToSelector:aSelector];
}

- (id)forwardingTargetForSelector:(SEL)aSelector {
  return [self.inner respondsToSelector:aSelector] ? self.inner : nil;
}

- (void)webView:(WKWebView *)webView
    decidePolicyForNavigationResponse:(WKNavigationResponse *)navigationResponse
                      decisionHandler:(void (^)(WKNavigationResponsePolicy))decisionHandler
    API_AVAILABLE(macos(11.3)) {
  // canShowMIMEType is false for application/zip, which is exactly the case
  // that used to fall on the floor.
  if (!navigationResponse.canShowMIMEType) {
    decisionHandler(WKNavigationResponsePolicyDownload);
    return;
  }
  if ([self.inner respondsToSelector:_cmd]) {
    [(id<WKNavigationDelegate>)self.inner webView:webView
                decidePolicyForNavigationResponse:navigationResponse
                                  decisionHandler:decisionHandler];
    return;
  }
  decisionHandler(WKNavigationResponsePolicyAllow);
}

- (void)webView:(WKWebView *)webView
    navigationResponse:(WKNavigationResponse *)navigationResponse
     didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
  download.delegate = self;
}

- (void)webView:(WKWebView *)webView
    navigationAction:(WKNavigationAction *)navigationAction
   didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
  download.delegate = self;
}

- (void)download:(WKDownload *)download
    decideDestinationUsingResponse:(NSURLResponse *)response
                 suggestedFilename:(NSString *)suggestedFilename
                 completionHandler:(void (^)(NSURL *destination))completionHandler
    API_AVAILABLE(macos(11.3)) {
  NSURL *dir = [NSFileManager.defaultManager URLsForDirectory:NSDownloadsDirectory
                                                    inDomains:NSUserDomainMask]
                   .firstObject;
  if (dir == nil) {
    completionHandler(nil);
    return;
  }
  NSString *base = suggestedFilename.stringByDeletingPathExtension;
  NSString *ext = suggestedFilename.pathExtension;
  NSURL *dst = [dir URLByAppendingPathComponent:suggestedFilename];
  // Non-clobbering: exporting twice must not quietly replace the copy the
  // author already put somewhere.
  for (int n = 2; [NSFileManager.defaultManager fileExistsAtPath:dst.path]; n++) {
    NSString *name = [NSString stringWithFormat:@"%@ %d.%@", base, n, ext];
    dst = [dir URLByAppendingPathComponent:name];
  }
  completionHandler(dst);
}

- (void)downloadDidFinish:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
  NSURL *url = download.progress.fileURL;
  if (url != nil) {
    [NSWorkspace.sharedWorkspace activateFileViewerSelectingURLs:@[ url ]];
  }
  appkitDownloadFinished(1);
}

- (void)download:(WKDownload *)download didFailWithError:(NSError *)error
       resumeData:(NSData *)resumeData API_AVAILABLE(macos(11.3)) {
  appkitDownloadFinished(0);
}

@end

static AppKitDownloadDelegate *gDownloadDelegate = nil;

static void downloadsNow(int attemptsLeft) {
  if (gDownloadDelegate == nil) {
    gDownloadDelegate = [AppKitDownloadDelegate new];
  }
  __block BOOL wired = NO;
  BOOL found = forEachWindowWebView(^(WKWebView *webView) {
    if (webView.navigationDelegate == (id)gDownloadDelegate) {
      wired = YES;
      return;
    }
    gDownloadDelegate.inner = webView.navigationDelegate;
    webView.navigationDelegate = gDownloadDelegate;
    wired = YES;
  });
  if (found && wired) {
    appkitDownloadsInstalled(1);
    return;
  }
  if (attemptsLeft > 0) {
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.25 * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
                     downloadsNow(attemptsLeft - 1);
                   });
    return;
  }
  appkitDownloadsInstalled(0);
}

void appkit_install_downloads(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    downloadsNow(8);
  });
}
