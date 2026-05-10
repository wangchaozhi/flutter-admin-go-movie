import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import 'login_storage.dart';
import 'widgets/login_header.dart';

class MobileLoginPage extends StatefulWidget {
  const MobileLoginPage({super.key});

  @override
  State<MobileLoginPage> createState() => _MobileLoginPageState();
}

class _MobileLoginPageState extends State<MobileLoginPage> {
  final _storage = LoginStorage();
  final _usernameController = TextEditingController(text: 'user');
  final _passwordController = TextEditingController();

  bool _loading = false;
  bool _remember = false;
  bool _ready = false;
  String _usernameError = '';
  String _passwordError = '';

  @override
  void initState() {
    super.initState();
    _loadSavedLogin();
  }

  @override
  void dispose() {
    _usernameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _loadSavedLogin() async {
    final saved = await _storage.load();
    if (!mounted) return;

    _usernameController.text = saved.username;
    _passwordController.text = saved.password;
    setState(() {
      _remember = saved.remember;
      _ready = true;
    });
  }

  Future<void> _login() async {
    if (!_validate()) return;

    setState(() => _loading = true);
    try {
      final username = _usernameController.text.trim();
      final password = _passwordController.text;
      final resp = await ApiClient().post('/api/mobile/login', {
        'username': username,
        'password': password,
      });

      if (!mounted) return;
      if (resp['code'] != 0) {
        _showMessage(resp['msg']?.toString() ?? '登录失败');
        return;
      }

      await _storage.save(
        username: username,
        password: password,
        remember: _remember,
      );
      if (!mounted) return;
      Navigator.pushReplacementNamed(context, '/home');
    } catch (e) {
      if (!mounted) return;
      _showMessage('登录失败: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  bool _validate() {
    final usernameError = _usernameController.text.trim().isEmpty ? '请输入用户名' : '';
    final passwordError = _passwordController.text.isEmpty ? '请输入密码' : '';
    setState(() {
      _usernameError = usernameError;
      _passwordError = passwordError;
    });
    return usernameError.isEmpty && passwordError.isEmpty;
  }

  void _showMessage(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (!_ready) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }

    return Scaffold(
      body: Stack(
        children: [
          const _LoginBackground(),
          SafeArea(
            child: Center(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(20),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 430),
                  child: _LoginCard(
                    usernameController: _usernameController,
                    passwordController: _passwordController,
                    usernameError: _usernameError,
                    passwordError: _passwordError,
                    remember: _remember,
                    loading: _loading,
                    onRememberChanged: (value) => setState(() => _remember = value),
                    onLogin: _login,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _LoginBackground extends StatelessWidget {
  const _LoginBackground();

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          colors: [Color(0xFFF8FAFC), Color(0xFFEFF6FF), Color(0xFFF0FDFA)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
      ),
      child: Stack(
        children: [
          Positioned(
            top: -90,
            right: -80,
            child: _BlurCircle(color: const Color(0x332563EB), size: 220),
          ),
          Positioned(
            bottom: -70,
            left: -70,
            child: _BlurCircle(color: const Color(0x3314B8A6), size: 190),
          ),
        ],
      ),
    );
  }
}

class _BlurCircle extends StatelessWidget {
  const _BlurCircle({required this.color, required this.size});

  final Color color;
  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(shape: BoxShape.circle, color: color),
    );
  }
}

class _LoginCard extends StatelessWidget {
  const _LoginCard({
    required this.usernameController,
    required this.passwordController,
    required this.usernameError,
    required this.passwordError,
    required this.remember,
    required this.loading,
    required this.onRememberChanged,
    required this.onLogin,
  });

  final TextEditingController usernameController;
  final TextEditingController passwordController;
  final String usernameError;
  final String passwordError;
  final bool remember;
  final bool loading;
  final ValueChanged<bool> onRememberChanged;
  final VoidCallback onLogin;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.white.withValues(alpha: 0.94),
      elevation: 18,
      shadowColor: const Color(0x1F0F172A),
      borderRadius: BorderRadius.circular(28),
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const LoginHeader(),
            const SizedBox(height: 26),
            TextField(
              controller: usernameController,
              textInputAction: TextInputAction.next,
              decoration: _fieldDecoration(
                label: '用户名',
                hint: '请输入用户名',
                icon: Icons.person_outline_rounded,
                error: usernameError,
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: passwordController,
              obscureText: true,
              textInputAction: TextInputAction.done,
              onSubmitted: (_) {
                if (!loading) onLogin();
              },
              decoration: _fieldDecoration(
                label: '密码',
                hint: '请输入密码',
                icon: Icons.lock_outline_rounded,
                error: passwordError,
              ),
            ),
            const SizedBox(height: 16),
            SwitchListTile.adaptive(
              value: remember,
              contentPadding: EdgeInsets.zero,
              title: const Text('记住密码'),
              subtitle: const Text('下次打开自动回填账号和密码'),
              onChanged: loading ? null : onRememberChanged,
            ),
            const SizedBox(height: 18),
            FilledButton.icon(
              onPressed: loading ? null : onLogin,
              icon: loading
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.arrow_forward_rounded),
              label: Text(loading ? '登录中...' : '登录'),
              style: FilledButton.styleFrom(
                minimumSize: const Size.fromHeight(52),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(16),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  InputDecoration _fieldDecoration({
    required String label,
    required String hint,
    required IconData icon,
    required String error,
  }) {
    return InputDecoration(
      labelText: label,
      hintText: hint,
      errorText: error.isEmpty ? null : error,
      prefixIcon: Icon(icon),
      filled: true,
      fillColor: const Color(0xFFF8FAFC),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: const BorderSide(color: Color(0xFFE5E7EB)),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: const BorderSide(color: Color(0xFFE5E7EB)),
      ),
    );
  }
}
