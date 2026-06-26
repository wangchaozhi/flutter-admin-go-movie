import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/session.dart';

/// Self-service sign-up. On success the new account is signed straight in (the
/// backend returns a token) and the user lands on the home page.
class RegisterPage extends StatefulWidget {
  const RegisterPage({super.key});

  @override
  State<RegisterPage> createState() => _RegisterPageState();
}

class _RegisterPageState extends State<RegisterPage> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  final _nickname = TextEditingController();
  final _email = TextEditingController();

  bool _loading = false;
  String? _error;

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    _confirm.dispose();
    _nickname.dispose();
    _email.dispose();
    super.dispose();
  }

  String? _validate() {
    final username = _username.text.trim();
    if (username.length < 3 || username.length > 32) return '用户名长度需为 3-32 个字符';
    if (_password.text.length < 6) return '密码至少 6 位';
    if (_password.text != _confirm.text) return '两次输入的密码不一致';
    final email = _email.text.trim();
    if (email.isNotEmpty && !email.contains('@')) return '邮箱格式不正确';
    return null;
  }

  Future<void> _register() async {
    final problem = _validate();
    if (problem != null) {
      setState(() => _error = problem);
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final username = _username.text.trim();
      final resp = await ApiClient().post('/api/mobile/register', {
        'username': username,
        'password': _password.text,
        'nickname': _nickname.text.trim(),
        'email': _email.text.trim(),
      });
      if (!mounted) return;
      if (resp['code'] != 0) {
        setState(() => _error = resp['msg']?.toString() ?? '注册失败');
        return;
      }
      final data = resp['data'] as Map<String, dynamic>?;
      final token = data?['token'] as String? ?? '';
      await Session.save(token, username);
      if (!mounted) return;
      Navigator.pushNamedAndRemoveUntil(context, '/home', (route) => false);
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = '注册失败: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0D0F14),
        foregroundColor: Colors.white,
        elevation: 0,
        title: const Text('注册账号'),
      ),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 430),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _field(_username, '用户名', '3-32 个字符', Icons.person_outline_rounded),
                  const SizedBox(height: 14),
                  _field(_password, '密码', '至少 6 位', Icons.lock_outline_rounded, obscure: true),
                  const SizedBox(height: 14),
                  _field(_confirm, '确认密码', '再次输入密码', Icons.lock_outline_rounded, obscure: true),
                  const SizedBox(height: 14),
                  _field(_nickname, '昵称（可选）', '展示名称', Icons.badge_outlined),
                  const SizedBox(height: 14),
                  _field(_email, '邮箱（可选）', 'name@example.com', Icons.mail_outline_rounded),
                  if (_error != null) ...[
                    const SizedBox(height: 14),
                    Text(
                      _error!,
                      style: const TextStyle(color: Color(0xFFFCA5A5), fontSize: 13),
                    ),
                  ],
                  const SizedBox(height: 22),
                  FilledButton(
                    onPressed: _loading ? null : _register,
                    style: FilledButton.styleFrom(
                      backgroundColor: const Color(0xFF25D0AB),
                      foregroundColor: const Color(0xFF07110F),
                      minimumSize: const Size.fromHeight(52),
                      textStyle: const TextStyle(fontWeight: FontWeight.w900),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                    ),
                    child: _loading
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2, color: Color(0xFF101318)),
                          )
                        : const Text('注册并登录'),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _field(
    TextEditingController controller,
    String label,
    String hint,
    IconData icon, {
    bool obscure = false,
  }) {
    return TextField(
      controller: controller,
      obscureText: obscure,
      enableSuggestions: !obscure,
      autocorrect: !obscure,
      cursorColor: const Color(0xFF25D0AB),
      style: const TextStyle(color: Colors.white),
      decoration: InputDecoration(
        labelText: label,
        hintText: hint,
        prefixIcon: Icon(icon),
        prefixIconColor: const Color(0xFF25D0AB),
        filled: true,
        fillColor: const Color(0xFF101318),
        labelStyle: const TextStyle(color: Color(0xFF9CA3AF)),
        hintStyle: const TextStyle(color: Color(0xFF6B7280)),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: Color(0xFF2B3140)),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: Color(0xFF2B3140)),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: Color(0xFF25D0AB), width: 1.4),
        ),
      ),
    );
  }
}
