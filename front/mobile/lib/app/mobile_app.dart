import 'package:flutter/material.dart';
import 'package:forui/forui.dart';

import '../core/session.dart';
import '../features/auth/mobile_login_page.dart';
import '../features/home/mobile_home_page.dart';
import '../features/video/video_player_page.dart';
import '../models/video.dart' as model;

class MobileApp extends StatelessWidget {
  const MobileApp({super.key});

  @override
  Widget build(BuildContext context) {
    final foruiTheme = FThemes.zinc.light.touch;

    return MaterialApp(
      title: 'Go Movie',
      debugShowCheckedModeBanner: false,
      localizationsDelegates: FLocalizations.localizationsDelegates,
      supportedLocales: FLocalizations.supportedLocales,
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
        '/home': (_) => const MobileHomePage(),
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
    if (tok != null && tok.isNotEmpty) {
      Navigator.pushReplacementNamed(context, '/home');
    } else {
      Navigator.pushReplacementNamed(context, '/login');
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
