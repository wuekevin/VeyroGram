# Regenerates Probe.xcodeproj (needs the xcodeproj gem that ships with CocoaPods): GEM_HOME=<cocoapods libexec> <cocoapods ruby> gen_project.rb
require 'fileutils'
require 'xcodeproj'
path = File.join(__dir__, 'Probe.xcodeproj')
FileUtils.rm_rf(path)
proj = Xcodeproj::Project.new(path)
t = proj.new_target(:application, 'Probe', :ios, '13.0')
g = proj.main_group.new_group('Probe', 'Probe')
t.add_file_references([g.new_file('main.m')])
g.new_file('Info.plist')
t.build_configurations.each do |c|
  c.build_settings.merge!('PRODUCT_BUNDLE_IDENTIFIER' => 'app.veyrogram.improbe', 'INFOPLIST_FILE' => 'Probe/Info.plist', 'PRODUCT_NAME' => 'Probe',
                          'CODE_SIGNING_ALLOWED' => 'NO', 'CODE_SIGNING_REQUIRED' => 'NO', 'TARGETED_DEVICE_FAMILY' => '1', 'CLANG_ENABLE_OBJC_ARC' => 'YES',
                          'OTHER_LDFLAGS' => ['$(inherited)', '-lc++'], 'SDKROOT' => 'iphoneos', 'SUPPORTED_PLATFORMS' => 'iphonesimulator iphoneos')
end
proj.recreate_user_schemes
proj.save
Xcodeproj::XCScheme.share_scheme(path, 'Probe') rescue nil
