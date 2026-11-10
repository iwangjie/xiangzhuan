// Keeps 香篆 in the menu bar only, or in the Dock, following the app's
// `hideDockIcon` storage key. nsh's runner calls
// `-[NSApplication setActivationPolicy:]` with nsh.json's policy; this swizzle
// replaces that policy with Accessory while hiding is on, and toggles live by
// polling the app's storage.json (default: hidden, also absent the key).
//
// It also owns the rest lock. The host writes `restActive` and `restLock`
// into the same storage file. While `restLock` is "1" the dylib raises the
// rest windows above the menu bar, disables app switching and swallows
// keyboard/mouse events; when `restActive` drops it restores the options and
// the window levels and activates the app that was in front before the rest.
#import <AppKit/AppKit.h>
#import <objc/runtime.h>

static NSString *bundleId;
static NSString *prefPath;
static BOOL hideDock = YES;
static NSTimer *pollTimer;

// MARK: Dock policy

static BOOL (*origSetPolicy)(id, SEL, NSInteger);

static NSDictionary *readStorage(void) {
  NSData *data = [NSData dataWithContentsOfFile:prefPath];
  if (!data) return nil;
  id value = [NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];
  return [value isKindOfClass:NSDictionary.class] ? value : nil;
}

static void applyPolicy(void) {
  NSApplication *app = NSApplication.sharedApplication;
  NSInteger policy = hideDock ? NSApplicationActivationPolicyAccessory
                              : NSApplicationActivationPolicyRegular;
  if (origSetPolicy) origSetPolicy(app, @selector(setActivationPolicy:), policy);
}

static void refreshDock(NSDictionary *storage) {
  if (!storage) return;
  id value = storage[@"hideDockIcon"];
  BOOL next = !([value isKindOfClass:NSString.class] && [value isEqualToString:@"0"]);
  if (next == hideDock) return;
  hideDock = next;
  dispatch_async(dispatch_get_main_queue(), ^{
    applyPolicy();
  });
}

static BOOL swizzledSetPolicy(id self, SEL _cmd, NSInteger policy) {
  if (hideDock) {
    return origSetPolicy(self, _cmd, NSApplicationActivationPolicyAccessory);
  }
  return origSetPolicy(self, _cmd, policy);
}

// MARK: Rest lock

static BOOL lockApplied = NO;
static BOOL lockSuppressed = NO;
static BOOL restActive = NO;
static BOOL restWantsLock = NO;
static NSUInteger savedPresentationOptions = 0;
static NSTimeInterval lockStartedAt = 0;
static NSTimeInterval lastRaiseAt = 0;
static NSMapTable<NSWindow *, NSNumber *> *savedLevels;
static NSMapTable<NSWindow *, NSNumber *> *savedBehaviors;
static id keyMonitor;
static id mouseMonitor;

static NSRunningApplication *lastFrontmost;
static NSRunningApplication *appBeforeOurs;
static NSTimeInterval oursActivatedAt = 0;
static NSRunningApplication *returnApp;

static NSTimeInterval uptime(void) {
  return NSProcessInfo.processInfo.systemUptime;
}

static BOOL isOurs(NSRunningApplication *app) {
  return app && app.processIdentifier == NSProcessInfo.processInfo.processIdentifier;
}

static BOOL isRestWindow(NSWindow *win) {
  if (!win.isVisible || win.isMiniaturized) return NO;
  if ([win.title containsString:@"休息"]) return YES;
  if (win.styleMask == NSWindowStyleMaskBorderless || win.level > NSNormalWindowLevel) {
    for (NSScreen *screen in NSScreen.screens) {
      NSRect frame = win.frame;
      NSRect screenFrame = screen.frame;
      if (fabs(NSWidth(frame) - NSWidth(screenFrame)) < 2.0 &&
          fabs(NSHeight(frame) - NSHeight(screenFrame)) < 60.0) {
        return YES;
      }
    }
  }
  return NO;
}

static BOOL hasVisibleRestWindow(void) {
  for (NSWindow *win in NSApplication.sharedApplication.windows) {
    if (isRestWindow(win)) return YES;
  }
  return NO;
}

static void raiseRestWindows(void) {
  NSApplication *app = NSApplication.sharedApplication;
  NSTimeInterval now = uptime();
  BOOL force = !app.isActive || now - lastRaiseAt >= 1.0;
  for (NSWindow *win in app.windows) {
    if (!isRestWindow(win)) continue;
    if ([savedLevels objectForKey:win] == nil) {
      [savedLevels setObject:@(win.level) forKey:win];
      [savedBehaviors setObject:@(win.collectionBehavior) forKey:win];
      force = YES;
    }
    win.level = NSScreenSaverWindowLevel;
    win.collectionBehavior |= NSWindowCollectionBehaviorCanJoinAllSpaces |
                              NSWindowCollectionBehaviorStationary |
                              NSWindowCollectionBehaviorFullScreenAuxiliary |
                              NSWindowCollectionBehaviorIgnoresCycle;
    if (force) [win orderFrontRegardless];
  }
  if (force) {
    lastRaiseAt = now;
    if (!app.isActive) [app activateIgnoringOtherApps:YES];
  }
}

static void applyRestLock(BOOL on) {
  if (on == lockApplied) return;
  NSApplication *app = NSApplication.sharedApplication;
  if (on) {
    if (!hasVisibleRestWindow()) return;
    lockApplied = YES;
    lockStartedAt = uptime();
    lastRaiseAt = 0;
    savedPresentationOptions = app.presentationOptions;
    NSApplicationPresentationOptions options = savedPresentationOptions;
    options &= ~(NSApplicationPresentationAutoHideDock |
                 NSApplicationPresentationAutoHideMenuBar);
    options |= NSApplicationPresentationHideDock |
               NSApplicationPresentationHideMenuBar |
               NSApplicationPresentationDisableAppleMenu |
               NSApplicationPresentationDisableProcessSwitching |
               NSApplicationPresentationDisableHideApplication;
    [app activateIgnoringOtherApps:YES];
    app.presentationOptions = options;
    raiseRestWindows();
    return;
  }

  lockApplied = NO;
  app.presentationOptions = savedPresentationOptions;
  for (NSWindow *win in savedLevels) {
    if (!win) continue;
    NSNumber *level = [savedLevels objectForKey:win];
    NSNumber *behavior = [savedBehaviors objectForKey:win];
    if (level) win.level = level.integerValue;
    if (behavior) win.collectionBehavior = behavior.unsignedIntegerValue;
  }
  [savedLevels removeAllObjects];
  [savedBehaviors removeAllObjects];
}

static void captureReturnApp(void) {
  returnApp = nil;
  NSTimeInterval now = uptime();
  if (now - oursActivatedAt < 5.0 && appBeforeOurs && !appBeforeOurs.isTerminated &&
      !isOurs(appBeforeOurs)) {
    returnApp = appBeforeOurs;
    return;
  }
  NSRunningApplication *front = NSWorkspace.sharedWorkspace.frontmostApplication;
  if (front && !front.isTerminated && !isOurs(front)) {
    returnApp = front;
  } else if (lastFrontmost && !lastFrontmost.isTerminated && !isOurs(lastFrontmost)) {
    // The rest window may have taken focus before the activation notification
    // reached us; `lastFrontmost` still holds the app it took it from.
    returnApp = lastFrontmost;
  }
}

static void restoreReturnApp(void) {
  NSRunningApplication *target = returnApp;
  returnApp = nil;
  if (!target || target.isTerminated) return;
  // In soft mode the user may have switched away during the rest; leave that
  // app alone instead of pulling focus back to where the rest started. The
  // activation waits a run loop so the lock's presentation options are gone.
  dispatch_async(dispatch_get_main_queue(), ^{
    if (!isOurs(NSWorkspace.sharedWorkspace.frontmostApplication)) return;
    [target activateWithOptions:NSApplicationActivateAllWindows];
  });
}

static void installActivationTracking(void) {
  lastFrontmost = NSWorkspace.sharedWorkspace.frontmostApplication;
  [[NSWorkspace sharedWorkspace].notificationCenter
      addObserverForName:NSWorkspaceDidActivateApplicationNotification
                  object:nil
                   queue:NSOperationQueue.mainQueue
              usingBlock:^(NSNotification *note) {
                NSRunningApplication *app = note.userInfo[NSWorkspaceApplicationKey];
                if (!app) return;
                if (isOurs(app)) {
                  if (lastFrontmost && !isOurs(lastFrontmost)) appBeforeOurs = lastFrontmost;
                  oursActivatedAt = uptime();
                }
                lastFrontmost = app;
              }];
}

static void installInputMonitors(void) {
  NSEventMask keyMask = NSEventMaskKeyDown | NSEventMaskKeyUp |
                        NSEventMaskFlagsChanged | NSEventMaskSystemDefined;
  keyMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:keyMask
                                                     handler:^NSEvent *(NSEvent *event) {
                                                       return lockApplied ? nil : event;
                                                     }];
  NSEventMask mouseMask = NSEventMaskLeftMouseDown | NSEventMaskLeftMouseUp |
                          NSEventMaskRightMouseDown | NSEventMaskRightMouseUp |
                          NSEventMaskOtherMouseDown | NSEventMaskOtherMouseUp |
                          NSEventMaskLeftMouseDragged | NSEventMaskRightMouseDragged |
                          NSEventMaskOtherMouseDragged | NSEventMaskMouseMoved |
                          NSEventMaskScrollWheel | NSEventMaskTabletPoint |
                          NSEventMaskTabletProximity | NSEventMaskGesture |
                          NSEventMaskMagnify | NSEventMaskSwipe | NSEventMaskRotate |
                          NSEventMaskBeginGesture | NSEventMaskEndGesture |
                          NSEventMaskCursorUpdate | NSEventMaskMouseEntered |
                          NSEventMaskMouseExited;
  mouseMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:mouseMask
                                                       handler:^NSEvent *(NSEvent *event) {
                                                         return lockApplied ? nil : event;
                                                       }];
}

static void refreshRest(NSDictionary *storage) {
  if (!storage) return;
  id activeValue = storage[@"restActive"];
  id lockValue = storage[@"restLock"];
  BOOL active = [activeValue isKindOfClass:NSString.class] && [activeValue isEqualToString:@"1"];
  BOOL wantsLock = [lockValue isKindOfClass:NSString.class] && [lockValue isEqualToString:@"1"];
  NSTimeInterval now = uptime();

  if (active && !restActive) {
    restActive = YES;
    lockSuppressed = NO;
    captureReturnApp();
  }

  if (!active) {
    if (restActive) {
      restActive = NO;
      if (lockApplied) applyRestLock(NO);
      restoreReturnApp();
    } else if (lockApplied) {
      applyRestLock(NO);
    }
    restWantsLock = NO;
    return;
  }

  restWantsLock = wantsLock;

  // A stuck rest must never trap the desktop: the longest configured rest is
  // 120 s plus the page's ~7 s outro (12 s fallback), so five minutes is a
  // safe ceiling.
  if (lockApplied && now - lockStartedAt > 300.0) {
    applyRestLock(NO);
    lockSuppressed = YES;
  }
  if (lockApplied && !hasVisibleRestWindow()) applyRestLock(NO);

  if (restWantsLock && !lockApplied && !lockSuppressed) {
    if (hasVisibleRestWindow()) applyRestLock(YES);
  } else if (!restWantsLock && lockApplied) {
    applyRestLock(NO);
  }

  if (lockApplied) raiseRestWindows();
}

// MARK: Install

__attribute__((constructor)) static void install(void) {
  Method method = class_getInstanceMethod([NSApplication class], @selector(setActivationPolicy:));
  if (method) {
    origSetPolicy = (void *)method_setImplementation(method, (IMP)swizzledSetPolicy);
  }

  bundleId = [[NSBundle mainBundle] bundleIdentifier];
  if (bundleId.length == 0) bundleId = @"com.iwangjie.xiangzhuan";
  prefPath = [NSHomeDirectory()
      stringByAppendingPathComponent:
          [NSString stringWithFormat:@"Library/Application Support/%@/storage.json", bundleId]];

  savedLevels = [NSMapTable weakToStrongObjectsMapTable];
  savedBehaviors = [NSMapTable weakToStrongObjectsMapTable];

  // The runner calls setActivationPolicy before any run loop; keep Accessory
  // from the start, then follow the stored preferences once the app is up.
  dispatch_async(dispatch_get_main_queue(), ^{
    installActivationTracking();
    installInputMonitors();

    NSDictionary *initial = readStorage();
    if (initial) {
      id value = initial[@"hideDockIcon"];
      hideDock = !([value isKindOfClass:NSString.class] && [value isEqualToString:@"0"]);
    }
    applyPolicy();

    pollTimer = [NSTimer timerWithTimeInterval:0.1
                                       repeats:YES
                                         block:^(NSTimer *timer) {
                                           (void)timer;
                                           NSDictionary *storage = readStorage();
                                           if (!storage) return;
                                           refreshDock(storage);
                                           refreshRest(storage);
                                         }];
    [[NSRunLoop mainRunLoop] addTimer:pollTimer forMode:NSRunLoopCommonModes];
  });
}
