import 'package:shared_preferences/shared_preferences.dart';

class SavedLogin {
  const SavedLogin({
    required this.username,
    required this.password,
    required this.remember,
  });

  final String username;
  final String password;
  final bool remember;
}

class LoginStorage {
  static const _rememberKey = 'mobile.remember';
  static const _usernameKey = 'mobile.username';
  static const _passwordKey = 'mobile.password';

  Future<SavedLogin> load() async {
    final prefs = await SharedPreferences.getInstance();
    final remember = prefs.getBool(_rememberKey) ?? true;

    return SavedLogin(
      username: prefs.getString(_usernameKey) ?? 'user',
      password: remember ? prefs.getString(_passwordKey) ?? '123456' : '123456',
      remember: remember,
    );
  }

  Future<void> save({
    required String username,
    required String password,
    required bool remember,
  }) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_rememberKey, remember);

    if (!remember) {
      await prefs.remove(_usernameKey);
      await prefs.remove(_passwordKey);
      return;
    }

    await prefs.setString(_usernameKey, username);
    await prefs.setString(_passwordKey, password);
  }
}
