// Keeps 香篆 in the menu bar only, or in the Dock, following the app's
// `hideDockIcon` storage key. nsh's release runner always calls
// `-[NSApplication setActivationPolicy:Regular]`; this swizzle replaces that
// policy with Accessory while hiding is on, and toggles live by polling the
// app's storage.json (default: hidden, also absent the key).
#import <AppKit/AppKit.h>
#import <objc/runtime.h>

static BOOL (*origSetPolicy)(id, SEL, NSInteger);
static BOOL hideDock = YES;
static NSString *prefPath;
static NSTimer *pollTimer;

static BOOL hasShownPreference(void) {
  NSString *content = [NSString stringWithContentsOfFile:prefPath
                                                encoding:NSUTF8StringEncoding
                                                   error:NULL];
  if (!content) return NO;
  return [content rangeOfString:@"\"hideDockIcon\":\"0\""].location != NSNotFound;
}

static void applyPolicy(void) {
  NSApplication *app = NSApplication.sharedApplication;
  NSInteger policy = hideDock ? NSApplicationActivationPolicyAccessory
                              : NSApplicationActivationPolicyRegular;
  if (origSetPolicy) origSetPolicy(app, @selector(setActivationPolicy:), policy);
}

static void refresh(void) {
  BOOL next = !hasShownPreference();
  if (next == hideDock) return;
  hideDock = next;
  dispatch_async(dispatch_get_main_queue(), ^{ applyPolicy(); });
}

static BOOL swizzledSetPolicy(id self, SEL _cmd, NSInteger policy) {
  if (hideDock) {
    return origSetPolicy(self, _cmd, NSApplicationActivationPolicyAccessory);
  }
  return origSetPolicy(self, _cmd, policy);
}

__attribute__((constructor)) static void install(void) {
  Method method = class_getInstanceMethod([NSApplication class], @selector(setActivationPolicy:));
  if (method) {
    origSetPolicy = (void *)method_setImplementation(method, (IMP)swizzledSetPolicy);
  }

  NSString *bundleId = [[NSBundle mainBundle] bundleIdentifier];
  if (bundleId.length == 0) bundleId = @"com.iwangjie.xiangzhuan";
  prefPath = [NSHomeDirectory()
      stringByAppendingPathComponent:
          [NSString stringWithFormat:@"Library/Application Support/%@/storage.json", bundleId]];

  // The runner calls setActivationPolicy before any run loop; keep Accessory
  // from the start, then follow the stored preference once the app is up.
  dispatch_async(dispatch_get_main_queue(), ^{
    hideDock = !hasShownPreference();
    applyPolicy();
    pollTimer = [NSTimer scheduledTimerWithTimeInterval:0.8
                                                repeats:YES
                                                  block:^(NSTimer *timer) {
                                                    refresh();
                                                  }];
  });
}
