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

  static Future<void> clear() async {
    final prefs = await _instance();
    await prefs.remove(_tokenKey);
    await prefs.remove(_usernameKey);
  }
}
