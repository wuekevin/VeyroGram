// One IM client (WuKongIMSDK, the exact source the iOS client ships). Launch arguments:
//   <host:port> <uid> <token> <connect|bad|recv|send> [peer] [text]
// Prints one JSON object per line on stdout ({"event":...}) and exits 0 on success, non-zero otherwise.
#import <UIKit/UIKit.h>
#import <WuKongIMSDK/WuKongIMSDK.h>

static void emit(NSDictionary *o) {
    NSData *d = [NSJSONSerialization dataWithJSONObject:o options:0 error:nil];
    printf("%s\n", [[[NSString alloc] initWithData:d encoding:NSUTF8StringEncoding] UTF8String]);
    fflush(stdout);
}

@interface Probe : UIResponder <UIApplicationDelegate, WKConnectionManagerDelegate, WKChatManagerDelegate>
@property(nonatomic, strong) UIWindow *window;
@property(nonatomic, copy) NSString *mode, *peer, *text;
@property(nonatomic) BOOL connected;
@end

@implementation Probe
- (BOOL)application:(UIApplication *)app didFinishLaunchingWithOptions:(NSDictionary *)opts {
    NSArray *a = NSProcessInfo.processInfo.arguments;
    if (a.count < 5) { emit(@{@"event": @"error", @"error": @"usage"}); exit(64); }
    NSString *addr = a[1], *uid = a[2], *token = a[3];
    self.mode = a[4]; self.peer = a.count > 5 ? a[5] : nil; self.text = a.count > 6 ? a[6] : nil;
    NSRange r = [addr rangeOfString:@":" options:NSBackwardsSearch];
    NSString *host = [addr substringToIndex:r.location]; uint16_t port = (uint16_t)[[addr substringFromIndex:r.location + 1] intValue];

    NSString *dir = [NSTemporaryDirectory() stringByAppendingPathComponent:[NSUUID UUID].UUIDString];
    [[NSFileManager defaultManager] createDirectoryAtPath:dir withIntermediateDirectories:YES attributes:nil error:nil];
    WKSDK *sdk = [WKSDK shared];
    sdk.options.host = host; sdk.options.port = port;
    sdk.options.dbDir = dir; sdk.options.messageFileRootDir = dir;
    sdk.options.connectInfoCallback = ^WKConnectInfo * _Nonnull {
        WKConnectInfo *c = [WKConnectInfo new]; c.uid = uid; c.token = token; return c;
    };
    // The app answers these over HTTP; this contract test has no business API, so it answers "nothing to sync".
    [sdk.conversationManager setSyncConversationProviderAndAck:^(long long version, NSString *lastMsgSeqs, WKSyncConversationCallback cb) {
        WKSyncConversationWrapModel *m = [WKSyncConversationWrapModel new]; m.conversations = @[]; cb(m, nil);
    } ack:^(uint64_t cmdVersion, void (^complete)(NSError *)) { complete(nil); }];
    [sdk setOfflineMessageProvider:^(int limit, uint32_t seq, WKOfflineMessageCallback cb) { cb(@[], false, nil); }
                offlineMessagesAck:^(uint32_t seq, void (^complete)(NSError *)) { complete(nil); }];
    [sdk.connectionManager addDelegate:self];
    [sdk.chatManager addDelegate:self];

    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(([self.mode isEqualToString:@"recv"] ? 60 : ([self.mode isEqualToString:@"bad"] ? 15 : 20)) * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
        if ([self.mode isEqualToString:@"bad"]) {
            emit(@{@"event": @"not_connected", @"connected": @(self.connected)});
            exit(self.connected ? 6 : 0);
        }
        emit(@{@"event": @"error", @"error": @"timeout"}); exit(2);
    });
    [sdk.connectionManager connect];
    return YES;
}

- (void)onConnectStatus:(WKConnectStatus)status reasonCode:(WKReason)reasonCode {
    emit(@{@"event": @"status", @"status": @(status), @"reasonCode": @(reasonCode)});
    if (status != WKConnected) return;
    self.connected = YES;
    emit(@{@"event": @"connected"});
    if ([self.mode isEqualToString:@"connect"]) { exit(0); }
    if ([self.mode isEqualToString:@"send"]) {
        [[WKSDK shared].chatManager sendMessage:[[WKTextContent alloc] initWithContent:self.text] channel:[WKChannel personWithChannelID:self.peer]];
    }
}
- (void)onSendack:(WKSendackPacket *)p left:(NSInteger)left {
    emit(@{@"event": @"ack", @"reasonCode": @(p.reasonCode)});
    if ([self.mode isEqualToString:@"send"]) exit(p.reasonCode == WK_REASON_SUCCESS ? 0 : 5);
}
- (void)onRecvMessages:(WKMessage *)m left:(NSInteger)left {
    NSString *t = [m.content isKindOfClass:WKTextContent.class] ? ((WKTextContent *)m.content).content : @"";
    emit(@{@"event": @"message", @"from": m.fromUid ?: @"", @"text": t ?: @""});
    if ([self.mode isEqualToString:@"recv"]) exit(0);
}
@end

int main(int argc, char *argv[]) {
    @autoreleasepool { return UIApplicationMain(argc, argv, nil, NSStringFromClass(Probe.class)); }
}
