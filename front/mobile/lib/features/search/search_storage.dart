import 'package:shared_preferences/shared_preferences.dart';

/// Persists the most recent search terms locally so the search page can offer
/// quick re-runs. Newest first, de-duplicated case-insensitively, capped.
class SearchHistoryStorage {
  static const _key = 'mobile.search.history';
  static const _max = 10;

  Future<List<String>> load() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getStringList(_key) ?? <String>[];
  }

  Future<List<String>> add(String term) async {
    final value = term.trim();
    if (value.isEmpty) return load();
    final prefs = await SharedPreferences.getInstance();
    final list = prefs.getStringList(_key) ?? <String>[];
    list.removeWhere((e) => e.toLowerCase() == value.toLowerCase());
    list.insert(0, value);
    while (list.length > _max) {
      list.removeLast();
    }
    await prefs.setStringList(_key, list);
    return list;
  }

  Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_key);
  }
}
