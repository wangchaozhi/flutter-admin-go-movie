import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:forui/forui.dart';

import '../core/api_client.dart';
import '../core/l10n/app_strings.dart';
import '../core/l10n/locale_controller.dart';
import '../core/session.dart';
import '../features/auth/mobile_login_page.dart';
import '../features/auth/register_page.dart';
import '../features/home/mobile_home_page.dart';
import '../features/payment/vip_page.dart';
import '../features/video/video_player_page.dart';
import '../models/video.dart' as model;

class MobileApp extends StatelessWidget {
  const MobileApp({super.key});

  // Lets the API layer route back to login from anywhere when the account is
  // banned mid-session, without holding a BuildContext.
  static final GlobalKey<NavigatorState> navigatorKey =
      GlobalKey<NavigatorState>();

  @override
  Widget build(BuildContext context) {
    final foruiTheme = FThemes.zinc.light.touch;

    // Forced logout (e.g. account banned while logged in): drop to the login
    // screen and explain why.
    ApiClient.onForcedLogout = (message) {
      final navigator = navigatorKey.currentState;
      if (navigator == null) return;
      navigator.pushNamedAndRemoveUntil('/login', (route) => false);
      final messenger = ScaffoldMessenger.maybeOf(navigator.context);
      messenger?.showSnackBar(
        SnackBar(content: Text(message), behavior: SnackBarBehavior.floating),
      );
    };

    // Rebuilds the whole app when the language changes so every Localizations
    // consumer picks up the new locale immediately.
    return ValueListenableBuilder<Locale>(
      valueListenable: localeController,
      builder: (context, locale, _) => MaterialApp(
        title: 'Go Movie',
        debugShowCheckedModeBanner: false,
        navigatorKey: navigatorKey,
        locale: locale,
        localizationsDelegates: [
          AppStrings.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
          ...FLocalizations.localizationsDelegates,
        ],
        supportedLocales: AppLocale.supportedLocales,
        theme: foruiTheme.toApproximateMaterialTheme().copyWith(
          colorScheme: ColorScheme.fromSeed(
            seedColor: const Color(0xFF25D0AB),
            brightness: Brightness.light,
          ),
          scaffoldBackgroundColor: const Color(0xFFF6F7FB),
        ),
        builder: (context, child) => FTheme(data: foruiTheme, child: child!),
        home: const _AuthGate(),
        routes: {
          '/login': (_) => const MobileLoginPage(),
          '/register': (_) => const RegisterPage(),
          '/home': (_) => const MobileHomePage(),
          '/vip': (_) => const VipPage(),
        },
        onGenerateRoute: (settings) {
          if (settings.name == '/player') {
            final video = settings.arguments as model.Video;
            return MaterialPageRoute(
              builder: (_) => VideoPlayerPage(video: video),
            );
          }
          return null;
        },
      ),
    );
  }
}

class _AuthGate extends StatefulWidget {
  const _AuthGate();

  @override
  State<_AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<_AuthGate> {
  @override
  void initState() {
    super.initState();
    _check();
  }

  Future<void> _check() async {
    final tok = await Session.token();
    if (!mounted) return;
    if (tok == null || tok.isEmpty) {
      Navigator.pushReplacementNamed(context, '/login');
      return;
    }
    // Validate the saved session before entering: a banned account makes the
    // profile call return ApiClient.bannedCode, whose handler routes to /login,
    // so we must not also push /home in that case.
    try {
      final resp = await ApiClient().getAuth('/api/mobile/profile');
      if (!mounted) return;
      if (resp['code'] == ApiClient.bannedCode) return;
      Navigator.pushReplacementNamed(context, '/home');
    } catch (_) {
      // Transient/network error: don't lock the user out over a failed check.
      if (mounted) Navigator.pushReplacementNamed(context, '/home');
    }
  }

  @override
  Widget build(BuildContext context) {
    return const Scaffold(
      backgroundColor: Color(0xFF0D0F14),
      body: Center(
        child: CircularProgressIndicator(color: Color(0xFF25D0AB)),
      ),
    );
  }
}
