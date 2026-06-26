import 'package:flutter/material.dart';
import 'package:media_kit/media_kit.dart';

import 'app/mobile_app.dart';
import 'core/l10n/locale_controller.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  MediaKit.ensureInitialized();
  // Restore the saved language before the first frame so the UI starts in the
  // user's chosen locale instead of flashing the default.
  await localeController.load();
  runApp(const MobileApp());
}
