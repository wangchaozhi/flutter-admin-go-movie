import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

class Session {
  static const _tokenKey = 'session.token';
  static const _usernameKey = 'session.username';

  // SharedPreferences.getInstance() resolves to the same singleton every time;
  // cache it so token lookups on each authed request don't await it repeatedly.
  static SharedPreferences? _prefs;

  static Future<SharedPreferences> _instance() async {
    return _prefs ??= await SharedPreferences.getInstance();
  }

  static Future<void> save(String token, String username) async {
    final prefs = await _instance();
    await prefs.setString(_tokenKey, token);
    await prefs.setString(_usernameKey, username);
  }

  static Future<String?> token() async {
    final prefs = await _instance();
    return prefs.getString(_tokenKey);
  }

  static Future<String?> username() async {
    final prefs = await _instance();
    return prefs.getString(_usernameKey);
  }

  /// Decodes the current user's id from the `uid` claim of the stored JWT.
  /// Returns null when signed out or the token can't be parsed. Used to tell
  /// which danmaku/comments belong to the viewer (the backend still enforces
  /// ownership on writes).
  static Future<int?> userId() async {
    final raw = await token();
    if (raw == null) return null;
    final parts = raw.split('.');
    if (parts.length != 3) return null;
    try {
      var payload = parts[1].replaceAll('-', '+').replaceAll('_', '/');
      while (payload.length % 4 != 0) {
        payload += '=';
      }
      final decoded = jsonDecode(utf8.decode(base64.decode(payload)));
      if (decoded is Map<String, dynamic>) {
        final uid = decoded['uid'];
        if (uid is int) return uid;
        if (uid is num) return uid.toInt();
      }
    } catch (_) {
      // Malformed token — treat as no identity.
    }
    return null;
  }

  static Future<void> clear() async {
    final prefs = await _instance();
    await prefs.remove(_tokenKey);
    await prefs.remove(_usernameKey);
  }
}
