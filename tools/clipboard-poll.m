// Tiny in-process bridge: unchanged pasteboards never enter the Go runtime.
// Build with tools/build-clipboard-poll.sh. All AppKit operations run on main.
#import <AppKit/AppKit.h>
#include <stdatomic.h>
#include <stdint.h>

@interface MaccyClipboardPoll : NSObject {
@public
    _Atomic(int64_t) accepted;
}
@property(nonatomic, strong) NSPasteboard *board;
@property(nonatomic, strong) NSTimer *timer;
@property(nonatomic, copy) void (^changed)(void);
- (void)schedule:(double)seconds;
@end

@implementation MaccyClipboardPoll
- (void)schedule:(double)seconds {
    [self.timer invalidate];
    __weak MaccyClipboardPoll *weakSelf = self;
    self.timer = [NSTimer timerWithTimeInterval:seconds repeats:YES block:^(NSTimer *timer) {
        MaccyClipboardPoll *poll = weakSelf;
        if (!poll) return;
        NSInteger count = poll.board.changeCount;
        if (count < 0 || count != atomic_load_explicit(&poll->accepted, memory_order_acquire)) {
            poll.changed();
        }
    }];
    [[NSRunLoop mainRunLoop] addTimer:self.timer forMode:NSRunLoopCommonModes];
}
@end

uint32_t maccy_poll_version(void) { return 1; }
void *maccy_poll_start(double seconds, int64_t count, NSPasteboard *board, void (^changed)(void)) {
    if (![NSThread isMainThread] || !changed || seconds < .05 || !isfinite(seconds)) return NULL;
    MaccyClipboardPoll *poll = [MaccyClipboardPoll new];
    poll.board = board ?: NSPasteboard.generalPasteboard;
    if (!poll.board) return NULL;
    atomic_init(&poll->accepted, count);
    poll.changed = changed;
    [poll schedule:seconds];
    return (__bridge_retained void *)poll;
}
int64_t *maccy_poll_accepted(void *handle) {
    return (int64_t *)&((__bridge MaccyClipboardPoll *)handle)->accepted;
}
void maccy_poll_interval(void *handle, double seconds) {
    if (![NSThread isMainThread] || !handle || seconds < .05 || !isfinite(seconds)) return;
    [(__bridge MaccyClipboardPoll *)handle schedule:seconds];
}
void maccy_poll_stop(void *handle) {
    if (![NSThread isMainThread] || !handle) return;
    MaccyClipboardPoll *poll = (__bridge_transfer MaccyClipboardPoll *)handle;
    [poll.timer invalidate];
    poll.timer = nil;
    poll.changed = nil;
}
