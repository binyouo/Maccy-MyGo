// Test-only AppKit probe. Never linked into release builds.
// It redirects generalPasteboard to an explicitly named test board and exposes
// commands over a private Unix socket. It does not read the system clipboard.
#import <AppKit/AppKit.h>
#import <objc/runtime.h>
#import <sys/socket.h>
#import <sys/un.h>
#import <sys/stat.h>
#import <unistd.h>
#import <mach-o/dyld.h>

static NSString *boardName;
static BOOL launched;
static NSPasteboard *testBoard(id self, SEL cmd) {
  return [NSPasteboard pasteboardWithName:boardName];
}

static NSString *textValue(id value) {
  if (!value || value == [NSNull null]) return @"";
  NSString *s = [value isKindOfClass:NSString.class] ? value : [value description];
  return s.length > 1200 ? [s substringToIndex:1200] : s;
}

static void tree(id node, NSMutableArray *out, NSMutableSet *seen, NSUInteger depth) {
  if (!node || depth > 45 || out.count >= 12000) return;
  NSValue *identity = [NSValue valueWithNonretainedObject:node];
  if ([seen containsObject:identity]) return;
  [seen addObject:identity];
  NSMutableDictionary *v = [NSMutableDictionary dictionary];
  @try {
    if ([node respondsToSelector:@selector(accessibilityRole)]) v[@"role"] = textValue([node accessibilityRole]);
    if ([node respondsToSelector:@selector(accessibilityLabel)]) v[@"label"] = textValue([node accessibilityLabel]);
    if ([node respondsToSelector:@selector(accessibilityTitle)]) v[@"title"] = textValue([node accessibilityTitle]);
    if ([node respondsToSelector:@selector(accessibilityHelp)]) v[@"help"] = textValue([node accessibilityHelp]);
    if ([node respondsToSelector:@selector(accessibilityValue)]) v[@"value"] = textValue([node accessibilityValue]);
    if ([node respondsToSelector:@selector(accessibilityAttributeValue:)]) {
      for (NSString *attribute in @[NSAccessibilityRoleAttribute,NSAccessibilityTitleAttribute,NSAccessibilityDescriptionAttribute,NSAccessibilityValueAttribute,NSAccessibilitySelectedAttribute]) {
        id value = [node accessibilityAttributeValue:attribute];
        if (value) v[attribute] = textValue(value);
      }
      for (id child in [node accessibilityAttributeValue:NSAccessibilityChildrenAttribute]) tree(child,out,seen,depth+1);
    }
    if ([node respondsToSelector:@selector(accessibilityFrame)]) {
      NSRect r = [node accessibilityFrame];
      v[@"frame"] = @[@(r.origin.x), @(r.origin.y), @(r.size.width), @(r.size.height)];
    }
    v[@"depth"] = @(depth);
    [out addObject:v];
    if ([node respondsToSelector:@selector(accessibilityChildren)]) {
      for (id child in [node accessibilityChildren]) tree(child,out,seen,depth+1);
    }
    // AppKit's hosting bridge sometimes exposes subviews but no AX children.
    if ([node isKindOfClass:NSView.class]) {
      for (NSView *child in [(NSView *)node subviews]) tree(child,out,seen,depth+1);
    }
  } @catch (NSException *exception) { v[@"error"] = exception.name; }
}

static NSWindow *panel(void) {
  for (NSWindow *w in NSApp.windows) if (w.visible && w.frame.size.width > 100 && ![w.className containsString:@"StatusBar"]) return w;
  return nil;
}

static void keepPanelOpen(void) {
  if (![NSProcessInfo.processInfo.environment[@"MACCY_BENCH_KEEP_OPEN"] isEqual:@"1"]) return;
  for (NSWindow *window in NSApp.windows) {
    if ([window isKindOfClass:NSPanel.class]) [(NSPanel *)window setHidesOnDeactivate:NO];
    if (![window.className containsString:@"FloatingPanel"]) continue;
    Method method = class_getInstanceMethod(window.class,@selector(resignKey));
    class_replaceMethod(window.class,@selector(resignKey),class_getMethodImplementation(NSPanel.class,@selector(resignKey)),method_getTypeEncoding(method));
    // UI-only measurements keep the panel visible while the user works on
    // the primary screen. Never used for idle/startup/capture measurements.
    Method hide = class_getInstanceMethod(window.class,@selector(orderOut:));
    class_replaceMethod(window.class,@selector(orderOut:),imp_implementationWithBlock(^(id self, id sender) {}),method_getTypeEncoding(hide));
  }
}

static NSDictionary *press(id node, NSString *needle, NSMutableSet *seen) {
  if (!node || seen.count > 12000) return nil;
  NSValue *identity=[NSValue valueWithNonretainedObject:node];
  if ([seen containsObject:identity]) return nil;
  [seen addObject:identity];
  @try {
    NSString *label=[node respondsToSelector:@selector(accessibilityLabel)] ? textValue([node accessibilityLabel]) : @"";
    NSString *help=[node respondsToSelector:@selector(accessibilityHelp)] ? textValue([node accessibilityHelp]) : @"";
    if (([label containsString:needle] || [help containsString:needle]) && [node respondsToSelector:@selector(accessibilityPerformPress)]) {
      BOOL ok=[node accessibilityPerformPress];
      if (!ok && [node respondsToSelector:@selector(accessibilityPerformAction:)]) {
        [node accessibilityPerformAction:NSAccessibilityPressAction]; ok=YES;
      }
      return @{@"found":@YES,@"ok":@(ok),@"label":label};
    }
    NSMutableArray *children=[NSMutableArray array];
    if ([node respondsToSelector:@selector(accessibilityChildren)]) [children addObjectsFromArray:[node accessibilityChildren] ?: @[]];
    if ([node respondsToSelector:@selector(accessibilityAttributeValue:)]) [children addObjectsFromArray:[node accessibilityAttributeValue:NSAccessibilityChildrenAttribute] ?: @[]];
    if ([node isKindOfClass:NSView.class]) [children addObjectsFromArray:[node subviews]];
    for (id child in children) { NSDictionary *result=press(child,needle,seen); if (result) return result; }
  } @catch(NSException *e) {}
  return nil;
}

static BOOL fill(id node, NSString *value, NSMutableSet *seen) {
  if (!node || seen.count>12000) return NO;
  NSValue *identity=[NSValue valueWithNonretainedObject:node];
  if ([seen containsObject:identity]) return NO; [seen addObject:identity];
  @try {
    NSString *role=[node respondsToSelector:@selector(accessibilityRole)] ? [node accessibilityRole] : nil;
    if ([node respondsToSelector:@selector(accessibilityAttributeValue:)]) role=[node accessibilityAttributeValue:NSAccessibilityRoleAttribute] ?: role;
    if ([role isEqual:NSAccessibilityTextFieldRole]) {
      if ([node respondsToSelector:@selector(accessibilitySetValue:forAttribute:)]) {
        [node accessibilitySetValue:value forAttribute:NSAccessibilityValueAttribute]; return YES;
      }
      if ([node respondsToSelector:@selector(setAccessibilityValue:)]) { [node setAccessibilityValue:value]; return YES; }
    }
    NSMutableArray *children=[NSMutableArray array];
    if ([node respondsToSelector:@selector(accessibilityChildren)]) [children addObjectsFromArray:[node accessibilityChildren] ?: @[]];
    if ([node respondsToSelector:@selector(accessibilityAttributeValue:)]) [children addObjectsFromArray:[node accessibilityAttributeValue:NSAccessibilityChildrenAttribute] ?: @[]];
    if ([node isKindOfClass:NSView.class]) [children addObjectsFromArray:[node subviews]];
    for (id child in children) if (fill(child,value,seen)) return YES;
  } @catch(NSException *e) {}
  return NO;
}

static NSEventModifierFlags modifiers(NSArray *names) {
  NSEventModifierFlags f = 0;
  for (NSString *s in names) {
    if ([s isEqual:@"command"]) f |= NSEventModifierFlagCommand;
    if ([s isEqual:@"option"]) f |= NSEventModifierFlagOption;
    if ([s isEqual:@"control"]) f |= NSEventModifierFlagControl;
    if ([s isEqual:@"shift"]) f |= NSEventModifierFlagShift;
  }
  return f;
}

static NSDictionary *handle(NSDictionary *request) {
  NSString *op = request[@"op"];
  if ([op isEqual:@"set"]) {
    [request[@"values"] enumerateKeysAndObjectsUsingBlock:^(id key, id value, BOOL *stop) {
      [NSUserDefaults.standardUserDefaults setObject:value forKey:key];
    }];
    [NSUserDefaults.standardUserDefaults synchronize];
    return @{@"ok":@YES};
  }
  if ([op isEqual:@"state"]) {
    keepPanelOpen();
    if ([request[@"tree"] boolValue]) {
      [NSApp accessibilitySetValue:@YES forAttribute:@"AXEnhancedUserInterface"];
    }
    NSMutableArray *windows = [NSMutableArray array];
    for (NSWindow *w in NSApp.windows) {
      NSRect r = w.frame;
      if (r.size.width < 20) continue;
      [windows addObject:@{@"id":@(w.windowNumber), @"class":w.className, @"key":@(w.keyWindow), @"title":w.title ?: @"", @"visible":@(w.visible), @"frame":@[@(r.origin.x),@(r.origin.y),@(r.size.width),@(r.size.height)]}];
    }
    NSMutableDictionary *settings = [NSMutableDictionary dictionary];
    for (NSString *key in @[@"popupScreen",@"popupPosition",@"showInStatusBar",@"ignoreEvents",@"historySize",@"windowSize"]) settings[key] = [NSUserDefaults.standardUserDefaults objectForKey:key] ?: NSNull.null;
    NSMutableDictionary *result = [@{@"launched":@(launched), @"windows":windows, @"defaults":settings, @"board":boardName, @"support":NSSearchPathForDirectoriesInDomains(NSApplicationSupportDirectory, NSUserDomainMask, YES).firstObject ?: @""} mutableCopy];
    if ([request[@"tree"] boolValue] && panel()) {
      NSMutableArray *nodes = [NSMutableArray array];
      tree(panel().contentView,nodes,[NSMutableSet set],0);
      result[@"nodes"] = nodes;
    }
    return result;
  }
  if ([op isEqual:@"open"]) {
    keepPanelOpen();
    [NSApp activateIgnoringOtherApps:YES];
    id delegate = NSApp.delegate;
    if ([delegate respondsToSelector:@selector(applicationShouldHandleReopen:hasVisibleWindows:)])
      [delegate applicationShouldHandleReopen:NSApp hasVisibleWindows:NO];
    return @{@"ok":@YES};
  }
  if ([op isEqual:@"close"]) { [panel() close]; return @{@"ok":@YES}; }
  if ([op isEqual:@"focus"]) { [NSApp activateIgnoringOtherApps:YES]; [panel() makeKeyAndOrderFront:nil]; return @{@"ok":@YES}; }
  if ([op isEqual:@"press"]) return press(panel().contentView,request[@"label"],[NSMutableSet set]) ?: @{@"found":@NO,@"ok":@NO};
  if ([op isEqual:@"fill"]) return @{@"ok":@(fill(panel().contentView,request[@"text"],[NSMutableSet set]))};
  if ([op isEqual:@"clear"]) {
    [NSApp activateIgnoringOtherApps:YES]; [panel() makeKeyAndOrderFront:nil];
    BOOL selected=[NSApp sendAction:@selector(selectAll:) to:nil from:nil];
    BOOL deleted=[NSApp sendAction:@selector(delete:) to:nil from:nil];
    return @{@"ok":@(selected && deleted)};
  }
  if ([op isEqual:@"key"] || [op isEqual:@"text"]) {
    NSWindow *w = panel();
    if (!w) return @{@"error":@"no visible window"};
    [NSApp activateIgnoringOtherApps:YES];
    [w makeKeyAndOrderFront:nil];
    NSString *chars = request[@"text"] ?: @"";
    if ([op isEqual:@"text"] && [w.firstResponder respondsToSelector:@selector(insertText:replacementRange:)]) {
      [(id)w.firstResponder insertText:chars replacementRange:NSMakeRange(NSNotFound,0)];
      return @{@"ok":@YES};
    }
    unsigned short code = [request[@"code"] unsignedShortValue];
    if ([op isEqual:@"key"]) {
      unichar c = [request[@"character"] unsignedShortValue];
      chars = [NSString stringWithCharacters:&c length:1];
    }
    NSEvent *e = [NSEvent keyEventWithType:NSEventTypeKeyDown location:NSZeroPoint modifierFlags:modifiers(request[@"modifiers"]) timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:w.windowNumber context:nil characters:chars charactersIgnoringModifiers:chars isARepeat:NO keyCode:code];
    NSEventModifierFlags flags=modifiers(request[@"modifiers"]);
    if (flags) {
      NSEvent *change=[NSEvent keyEventWithType:NSEventTypeFlagsChanged location:NSZeroPoint modifierFlags:flags timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:w.windowNumber context:nil characters:@"" charactersIgnoringModifiers:@"" isARepeat:NO keyCode:58];
      [NSApp postEvent:change atStart:NO];
    }
    [NSApp postEvent:e atStart:NO];
    NSEvent *up=[NSEvent keyEventWithType:NSEventTypeKeyUp location:NSZeroPoint modifierFlags:flags timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:w.windowNumber context:nil characters:chars charactersIgnoringModifiers:chars isARepeat:NO keyCode:code];
    [NSApp postEvent:up atStart:NO];
    if (flags) {
      NSEvent *change=[NSEvent keyEventWithType:NSEventTypeFlagsChanged location:NSZeroPoint modifierFlags:0 timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:w.windowNumber context:nil characters:@"" charactersIgnoringModifiers:@"" isARepeat:NO keyCode:58];
      [NSApp postEvent:change atStart:NO];
    }
    return @{@"ok":@YES};
  }
  if ([op isEqual:@"write"]) {
    NSPasteboard *pb = [NSPasteboard pasteboardWithName:boardName];
    [pb clearContents];
    NSMutableArray *items = [NSMutableArray array];
    for (NSArray *representations in request[@"items"]) {
      NSPasteboardItem *item = [NSPasteboardItem new];
      for (NSDictionary *representation in representations) {
        NSData *data;
        if (representation[@"file"]) data = [NSData dataWithContentsOfFile:representation[@"file"]];
        else if (representation[@"base64"]) data = [[NSData alloc] initWithBase64EncodedString:representation[@"base64"] options:0];
        else data = [textValue(representation[@"text"]) dataUsingEncoding:NSUTF8StringEncoding];
        if (data) [item setData:data forType:representation[@"type"]];
      }
      [items addObject:item];
    }
    BOOL ok = [pb writeObjects:items];
    return @{@"ok":@(ok), @"changeCount":@(pb.changeCount)};
  }
  if ([op isEqual:@"read"]) {
    NSMutableArray *items = [NSMutableArray array];
    for (NSPasteboardItem *item in [NSPasteboard pasteboardWithName:boardName].pasteboardItems) {
      NSMutableDictionary *representations = [NSMutableDictionary dictionary];
      for (NSString *type in item.types) representations[type] = [[item dataForType:type] base64EncodedStringWithOptions:0] ?: @"";
      [items addObject:representations];
    }
    return @{@"items":items};
  }
  if ([op isEqual:@"capture"]) {
    NSView *view = panel().contentView;
    if (!view) return @{@"error":@"no visible window"};
    NSBitmapImageRep *bitmap = [view bitmapImageRepForCachingDisplayInRect:view.bounds];
    [view cacheDisplayInRect:view.bounds toBitmapImageRep:bitmap];
    NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    return @{@"png":[png base64EncodedStringWithOptions:0] ?: @""};
  }
  if ([op isEqual:@"quit"]) { dispatch_async(dispatch_get_main_queue(), ^{ [NSApp terminate:nil]; }); return @{@"ok":@YES}; }
  return @{@"error":@"unknown command"};
}

__attribute__((constructor)) static void installProbe(void) {
  @autoreleasepool {
    NSDictionary *env = NSProcessInfo.processInfo.environment;
    // OCR/XPC children inherit DYLD_INSERT_LIBRARIES. Never replace their
    // pasteboard or bind/unlink the parent's control socket.
    NSString *expected=env[@"MACCY_BENCH_EXECUTABLE"];
    uint32_t pathSize=0; _NSGetExecutablePath(NULL,&pathSize);
    char *executable=malloc(pathSize);
    _NSGetExecutablePath(executable,&pathSize);
    NSString *actual=[[NSString stringWithUTF8String:executable] stringByResolvingSymlinksInPath]; free(executable);
    if (!expected.length || ![actual isEqual:[expected stringByResolvingSymlinksInPath]]) return;
    boardName = env[@"MACCY_BENCH_BOARD"];
    NSString *path = env[@"MACCY_BENCH_SOCKET"];
    if (!boardName.length || !path.length) return;
    Method method = class_getClassMethod(NSPasteboard.class, @selector(generalPasteboard));
    method_setImplementation(method,(IMP)testBoard);
    [NSNotificationCenter.defaultCenter addObserverForName:NSApplicationDidFinishLaunchingNotification object:nil queue:nil usingBlock:^(NSNotification *n) {
      dispatch_async(dispatch_get_main_queue(), ^{ launched = YES; });
    }];
    dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0), ^{
      @autoreleasepool {
        int server = socket(AF_UNIX,SOCK_STREAM,0);
        struct sockaddr_un address = { .sun_family = AF_UNIX };
        if (path.length >= sizeof(address.sun_path)) return;
        strlcpy(address.sun_path,path.fileSystemRepresentation,sizeof(address.sun_path));
        unlink(address.sun_path);
        mode_t previous = umask(0077);
        int bound = bind(server,(struct sockaddr *)&address,sizeof(address));
        umask(previous);
        if (bound != 0 || listen(server,4) != 0) { perror("bench socket"); return; }
        for (;;) {
          int client = accept(server,NULL,NULL);
          if (client < 0) continue;
          FILE *input = fdopen(dup(client),"r");
          char *line = NULL; size_t capacity = 0;
          while (getline(&line,&capacity,input) >= 0) {
            @autoreleasepool {
              NSData *data = [[NSString stringWithUTF8String:line] dataUsingEncoding:NSUTF8StringEncoding];
              NSDictionary *request = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
              __block NSDictionary *result;
              dispatch_sync(dispatch_get_main_queue(), ^{
                @try { result = handle(request); }
                @catch(NSException *e) { result = @{@"error":e.reason ?: e.name}; }
              });
              NSData *response = [NSJSONSerialization dataWithJSONObject:result ?: @{} options:0 error:nil];
              const char *bytes = response.bytes; size_t left = response.length;
              while (left) { ssize_t n = write(client,bytes,left); if (n <= 0) break; bytes += n; left -= n; }
              write(client,"\n",1);
            }
          }
          free(line); fclose(input); close(client);
        }
      }
    });
  }
}
