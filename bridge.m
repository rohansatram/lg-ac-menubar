#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import "bridge.h"

static void SetupMainMenu(void) {
    NSMenu *mainMenu = [[NSMenu alloc] init];
    
    // Application menu
    NSMenuItem *appMenuItem = [[NSMenuItem alloc] init];
    NSMenu *appMenu = [[NSMenu alloc] initWithTitle:@"LG AC"];
    [appMenu addItemWithTitle:@"Quit" action:@selector(terminate:) keyEquivalent:@"q"];
    [appMenuItem setSubmenu:appMenu];
    [mainMenu addItem:appMenuItem];

    // Edit menu (enables Cmd+C, Cmd+V, Cmd+X, Cmd+A, Cmd+Z in WKWebView)
    NSMenuItem *editMenuItem = [[NSMenuItem alloc] init];
    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Edit"];
    [editMenu addItemWithTitle:@"Undo" action:@selector(undo:) keyEquivalent:@"z"];
    [editMenu addItemWithTitle:@"Redo" action:@selector(redo:) keyEquivalent:@"Z"];
    [editMenu addItem:[NSMenuItem separatorItem]];
    [editMenu addItemWithTitle:@"Cut" action:@selector(cut:) keyEquivalent:@"x"];
    [editMenu addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];
    [editMenu addItemWithTitle:@"Paste" action:@selector(paste:) keyEquivalent:@"v"];
    [editMenu addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
    [editMenuItem setSubmenu:editMenu];
    [mainMenu addItem:editMenuItem];

    [NSApp setMainMenu:mainMenu];
}

@interface MenuBarController : NSObject <NSPopoverDelegate>
@property (strong, nonatomic) NSStatusItem *statusItem;
@property (strong, nonatomic) NSPopover *popover;
@property (strong, nonatomic) WKWebView *webView;
@property (copy, nonatomic) NSString *initialURL;
@end

@implementation MenuBarController

- (instancetype)initWithURL:(NSString *)url {
    self = [super init];
    if (self) {
        self.initialURL = url;
        [self setupUI];
    }
    return self;
}

- (void)setupUI {
    // 1. Create NSStatusItem in macOS menu bar
    self.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    
    NSStatusBarButton *button = self.statusItem.button;
    if (button) {
        button.title = @"❄️";
        button.target = self;
        button.action = @selector(togglePopover:);
    }

    // 2. Setup NSPopover
    self.popover = [[NSPopover alloc] init];
    self.popover.contentSize = NSMakeSize(340, 480);
    self.popover.behavior = NSPopoverBehaviorTransient;
    self.popover.animates = YES;
    self.popover.delegate = self;

    // 3. Setup WKWebView
    WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
    self.webView = [[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 340, 480) configuration:config];
    
    // Transparent background for macOS glass vibrancy
    [self.webView setValue:@NO forKey:@"drawsBackground"];

    NSViewController *vc = [[NSViewController alloc] init];
    vc.view = self.webView;
    self.popover.contentViewController = vc;

    // Load initial URL
    NSURL *targetURL = [NSURL URLWithString:self.initialURL];
    NSURLRequest *req = [NSURLRequest requestWithURL:targetURL];
    [self.webView loadRequest:req];
}

- (void)togglePopover:(id)sender {
    NSStatusBarButton *button = self.statusItem.button;
    if (!button) return;

    if (self.popover.isShown) {
        [self.popover performClose:sender];
    } else {
        [self.webView evaluateJavaScript:@"if(typeof loadDevices==='function')loadDevices();" completionHandler:nil];
        
        [self.popover showRelativeToRect:button.bounds ofView:button preferredEdge:NSRectEdgeMinY];
        
        // Activate app so keyboard shortcuts (Cmd+V, Cmd+C) work
        [NSApp activateIgnoringOtherApps:YES];
        NSWindow *popoverWindow = self.popover.contentViewController.view.window;
        [popoverWindow makeKeyAndOrderFront:nil];
        [popoverWindow makeFirstResponder:self.webView];
    }
}

@end

void StartMenuBar(const char *urlStr) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        SetupMainMenu();

        NSString *url = [NSString stringWithUTF8String:urlStr];
        MenuBarController *controller = [[MenuBarController alloc] initWithURL:url];

        // Keep reference alive
        (void)controller;

        [NSApp run];
    }
}
