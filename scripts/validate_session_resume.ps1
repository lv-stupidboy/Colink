# validate_session_resume.ps1
# ACP 原生 session/resume 功能验证脚本
# 测试流程：
# 1. 创建会话并发送 prompt
# 2. 记录 ACP session ID
# 3. 进程退出（模拟断连）
# 4. 使用 session/resume 恢复会话
# 5. 发送新 prompt 验证上下文是否保留

param(
    [string]$TestDir = "$env:TEMP\acp_test"
)

Write-Host "=== ACP Session/Resume 验证测试 ===" -ForegroundColor Cyan
Write-Host "测试目录: $TestDir"

# 创建测试目录
if (-not (Test-Path $TestDir)) {
    New-Item -ItemType Directory -Path $TestDir | Out-Null
}

# 定义 C# 代码用于 ACP 交互
$code = @'
using System;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;

public class AcpValidator
{
    public static void Main(string[] args)
    {
        string testDir = args[0];
        bool verbose = args.Length > 1 && args[1] == "verbose";
        
        Console.OutputEncoding = Encoding.UTF8;
        
        Console.WriteLine("=== Step 1: 创建新会话 ===");
        
        // 创建新 session
        var psi = new ProcessStartInfo
        {
            FileName = "cmd.exe",
            Arguments = "/c opencode acp",
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
            WorkingDirectory = testDir
        };

        string acpSessionId = null;
        string firstResponse = null;
        
        using (var proc1 = Process.Start(psi))
        {
            var sw1 = proc1.StandardInput;
            var sr1 = proc1.StandardOutput;
            var err1 = proc1.StandardError;
            
            // Initialize
            sw1.WriteLine(@"{""jsonrpc"":""2.0"",""id"":0,""method"":""initialize"",""params"":{""protocolVersion"":1,""clientCapabilities"":{""fs"":{""readTextFile"":true,""writeTextFile"":true},""terminal"":true}}}");
            sw1.Flush();
            var initResp = sr1.ReadLine();
            if (verbose) Console.WriteLine("Init: " + initResp);
            
            // New session
            sw1.WriteLine(@"{""jsonrpc"":""2.0"",""id"":1,""method"":""session/new"",""params"":{""cwd"":""" + testDir.Replace("\\", "\\\\") + @""",""mcpServers"":[]}}");
            sw1.Flush();
            var sessionResp = sr1.ReadLine();
            if (verbose) Console.WriteLine("Session: " + sessionResp);
            
            // Extract session ID
            var match = Regex.Match(sessionResp ?? "", @"""sessionId"":""([^""]+)""");
            if (match.Success)
            {
                acpSessionId = match.Groups[1].Value;
                Console.WriteLine("ACP Session ID: " + acpSessionId);
            }
            else
            {
                Console.WriteLine("ERROR: Failed to extract session ID");
                return;
            }
            
            // Send first prompt
            Console.WriteLine("发送第一轮 prompt: '你好，请记住我的名字是张三'");
            sw1.WriteLine(@"{""jsonrpc"":""2.0"",""id"":2,""method"":""session/prompt"",""params"":{""sessionId"":""" + acpSessionId + @""",""prompt"":[{""type"":""text"",""text"":""你好，请记住我的名字是张三，我会测试你是否能记住""}]}}");
            sw1.Flush();
            
            // Read responses
            var sb1 = new StringBuilder();
            for (int i = 0; i < 20; i++)
            {
                var line = sr1.ReadLine();
                if (line == null) break;
                if (verbose) Console.WriteLine("Response: " + line);
                
                if (line.Contains("agent_message_chunk") || line.Contains("agent_thought_chunk"))
                {
                    var textMatch = Regex.Match(line, @"""text"":""([^""]+)""");
                    if (textMatch.Success)
                    {
                        sb1.Append(textMatch.Groups[1].Value);
                    }
                }
                
                if (line.Contains(@"""id"":2") && line.Contains("result"))
                {
                    break;
                }
            }
            
            firstResponse = sb1.ToString();
            Console.WriteLine("第一轮响应: " + firstResponse.Substring(0, Math.Min(200, firstResponse.Length)) + "...");
            
            // Close process
            sw1.Close();
            proc1.WaitForExit(3000);
        }
        
        Console.WriteLine("\n=== Step 2: 进程退出（模拟断连） ===");
        Thread.Sleep(2000);
        
        Console.WriteLine("\n=== Step 3: 使用 session/resume 恢复会话 ===");
        
        // Resume session (新进程)
        using (var proc2 = Process.Start(psi))
        {
            var sw2 = proc2.StandardInput;
            var sr2 = proc2.StandardOutput;
            
            // Initialize
            sw2.WriteLine(@"{""jsonrpc"":""2.0"",""id"":0,""method"":""initialize"",""params"":{""protocolVersion"":1,""clientCapabilities"":{""fs"":{""readTextFile"":true,""writeTextFile"":true},""terminal"":true}}}");
            sw2.Flush();
            sr2.ReadLine();
            
            // SessionResume
            Console.WriteLine("发送 session/resume 请求...");
            sw2.WriteLine(@"{""jsonrpc"":""2.0"",""id"":1,""method"":""session/resume"",""params"":{""sessionId"":""" + acpSessionId + @""",""cwd"":""" + testDir.Replace("\\", "\\\\") + @""",""mcpServers"":[]}}");
            sw2.Flush();
            var resumeResp = sr2.ReadLine();
            if (verbose) Console.WriteLine("Resume: " + resumeResp);
            
            if (resumeResp.Contains("error"))
            {
                Console.WriteLine("ERROR: session/resume failed");
                Console.WriteLine(resumeResp);
                return;
            }
            
            Console.WriteLine("session/resume 成功");
            
            // Send second prompt
            Console.WriteLine("\n=== Step 4: 发送新 prompt 验证上下文 ===");
            Console.WriteLine("发送第二轮 prompt: '你还记得我的名字吗？'");
            sw2.WriteLine(@"{""jsonrpc"":""2.0"",""id"":2,""method"":""session/prompt"",""params"":{""sessionId"":""" + acpSessionId + @""",""prompt"":[{""type"":""text"",""text"":""你还记得我的名字吗？""}]}}");
            sw2.Flush();
            
            // Read responses
            var sb2 = new StringBuilder();
            for (int i = 0; i < 30; i++)
            {
                var line = sr2.ReadLine();
                if (line == null) break;
                if (verbose) Console.WriteLine("Response: " + line);
                
                if (line.Contains("agent_message_chunk") || line.Contains("agent_thought_chunk"))
                {
                    var textMatch = Regex.Match(line, @"""text"":""([^""]+)""");
                    if (textMatch.Success)
                    {
                        sb2.Append(textMatch.Groups[1].Value);
                    }
                }
                
                if (line.Contains(@"""id"":2") && line.Contains("result"))
                {
                    break;
                }
            }
            
            var secondResponse = sb2.ToString();
            Console.WriteLine("\n第二轮响应: " + secondResponse);
            
            // Verify
            Console.WriteLine("\n=== Step 5: 验证结果 ===");
            if (secondResponse.Contains("张三"))
            {
                Console.WriteLine("✅ SUCCESS: Agent 记住了名字 '张三'");
                Console.WriteLine("session/resume 功能验证成功！");
            }
            else
            {
                Console.WriteLine("⚠️ WARNING: Agent 可能没有记住名字");
                Console.WriteLine("响应内容中未找到 '张三'");
            }
            
            sw2.Close();
            proc2.WaitForExit(3000);
        }
    }
}
'@

# 编译并运行
Add-Type -TypeDefinition $code -Language CSharp

Write-Host ""
Write-Host "开始运行验证..." -ForegroundColor Green
[AcpValidator]::Main(@($TestDir))

Write-Host ""
Write-Host "=== 测试完成 ===" -ForegroundColor Cyan