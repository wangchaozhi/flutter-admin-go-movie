import 'package:flutter/widgets.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// The languages the app ships with. Adding one is a single entry here plus a
/// matching block in app_strings.dart — nothing else needs to change.
class AppLocale {
  const AppLocale(this.code, this.locale, this.label);

  final String code;
  final Locale locale;
  final String label;

  static const List<AppLocale> values = [
    AppLocale('zh', Locale('zh'), '简体中文'),
    AppLocale('en', Locale('en'), 'English'),
    AppLocale('zh-TW', Locale('zh', 'TW'), '繁體中文'),
    AppLocale('ja', Locale('ja'), '日本語'),
  ];

  static List<Locale> get supportedLocales =>
      values.map((item) => item.locale).toList();

  /// Normalizes a Locale to one of our codes ("zh-TW" keeps the region; the
  /// rest collapse to the language). Unknown locales fall back to zh.
  static String codeOf(Locale locale) {
    if (locale.languageCode == 'zh' &&
        (locale.countryCode == 'TW' || locale.countryCode == 'HK')) {
      return 'zh-TW';
    }
    for (final item in values) {
      if (item.locale.languageCode == locale.languageCode &&
          item.locale.countryCode == locale.countryCode) {
        return item.code;
      }
    }
    for (final item in values) {
      if (item.locale.languageCode == locale.languageCode) return item.code;
    }
    return 'zh';
  }

  static AppLocale byCode(String code) {
    for (final item in values) {
      if (item.code == code) return item;
    }
    return values.first;
  }
}

/// Holds the active locale and persists it. The app listens via
/// [ValueListenableBuilder] so a change rebuilds MaterialApp (and therefore
/// every Localizations consumer) immediately.
class LocaleController extends ValueNotifier<Locale> {
  LocaleController() : super(AppLocale.values.first.locale);

  static const _storageKey = 'app.locale';

  /// Loads the saved locale before the first frame. Call from main().
  Future<void> load() async {
    final prefs = await SharedPreferences.getInstance();
    final code = prefs.getString(_storageKey);
    if (code != null) value = AppLocale.byCode(code).locale;
  }

  Future<void> setLocale(Locale locale) async {
    value = locale;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_storageKey, AppLocale.codeOf(locale));
  }
}

/// App-wide singleton; loaded in main() and read by MobileApp.
final localeController = LocaleController();
