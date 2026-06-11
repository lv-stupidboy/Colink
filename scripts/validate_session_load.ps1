# validate_session_load.ps1
# ACP 原生 session/load 功能验证脚本
# 测试 session/load（回放历史）是否能恢复上下文

param(
    [string]$TestDir = "$env:TEMP\acp_test"
)

Write-Host "=== ACP Session/Load 验证测试 ===" -ForegroundColor Cyan

$code = @'
using System;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;

public class AcpLoadValidator
{
    public static void Main(string[] args)
    {
        string testDir = args[0];
        bool verbose = args.Length > 1 && args[1] == "verbose";
        
        Console.OutputEncoding = Encoding.UTF8;
        
        Console.WriteLine("=== Step 1: 创建新会话并对话 ===");
        
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
        
        // 第一轮：创建会话并对话
        using (var proc1 = Process.Start(psi))
        {
            var sw1 = proc1.StandardInput;
            var sr1 = proc1.StandardOutput;
            
            // Initialize
            sw1.WriteLine(@"{""jsonrpc"":""2.0"",""id"":0,""method"":""initialize"",""params"":{""protocolVersion"":1}}");
            sw1.Flush();
            sr1.ReadLine();
            
            // New session
            sw1.WriteLine(@"{""jsonrpc"":""2.0"",""id"":1,""method"":""session/new"",""params"":{""cwd"":""" + testDir.Replace("\\", "\\\\") + @""",""mcpServers"":[]}}");
            sw1.Flush();
            var sessionResp = sr1.ReadLine();
            
            var match = Regex.Match(sessionResp ?? "", @"""sessionId"":""([^""]+)""");
            if (match.Success)
            {
                acpSessionId = match.Groups[1].Value;
                Console.WriteLine("ACP Session ID: " + acpSessionId);
            }
            
            // 发送第一轮 prompt
            Console.WriteLine("发送 prompt: '你好，我的名字是张三'");
            sw1.WriteLine(@"{""jsonrpc"":""2.0"",""id"":2,""method"":""session/prompt"",""params"":{""sessionId"":""" + acpSessionId + @""",""prompt"":[{""type"":""text"",""text"":""你好，我的名字是张三""}]}}");
            sw1.Flush();
            
            // 等待响应完成
            for (int i = 0; i < 15; i++)
            {
                var line = sr1.ReadLine();
                if (verbose) Console.WriteLine(line);
                if (line != null && line.Contains(@"""id"":2") && line.Contains("result")) break;
            }
            
            Console.WriteLine("第一轮对话完成");
            
            // 发送第二轮 prompt（增加更多上下文）
            Console.WriteLine("发送 prompt: '我们正在讨论 session/load 功能'");
            sw1.WriteLine(@"{""jsonrpc"":""2.0"",""id"":3,""method"":""session/prompt"",""params"":{""sessionId"":""" + acpSessionId + @""",""prompt"":[{""type"":""text"",""text"":""我们正在讨论 session/load 功能""}]}}");
            sw1.Flush();
            
            for (int i = 0; i < 15; i++)
            {
                var line = sr1.ReadLine();
                if (verbose) Console.WriteLine(line);
                if (line != null && line.Contains(@"""id"":3") && line.Contains("result")) break;
            }
            
            Console.WriteLine("第二轮对话完成");
            
            sw1.Close();
            proc1.WaitForExit(3000);
        }
        
        Console.WriteLine("\n=== Step 2: 进程退出 ===");
        Thread.Sleep(2000);
        
        Console.WriteLine("\n=== Step 3: 使用 session/load 恢复（回放历史） ===");
        
        // session/load
        using (var proc2 = Process.Start(psi))
        {
            var sw2 = proc2.StandardInput;
            var sr2 = proc2.StandardOutput;
            
            // Initialize
            sw2.WriteLine(@"{""jsonrpc"":""2.0"",""id"":0,""method"":""initialize"",""params"":{""protocolVersion"":1}}");
            sw2.Flush();
            sr2.ReadLine();
            
            // SessionLoad（回放历史）
            Console.WriteLine("发送 session/load 请求...");
            sw2.WriteLine(@"{""jsonrpc"":""2.0"",""id"":1,""method"":""session/load"",""params"":{""sessionId"":""" + acpSessionId + @""",""cwd"":""" + testDir.Replace("\\", "\\\\") + @""",""mcpServers"":[]}}");
            sw2.Flush();
            
            // 读取历史回放（session/update notifications）
            Console.WriteLine("等待历史回放...");
            int historyCount = 0;
            for (int i = 0; i < 50; i++)
            {
                var line = sr2.ReadLine();
                if (line == null) break;
                if (verbose) Console.WriteLine("History: " + line);
                
                if (line.Contains("session/update"))
                {
                    historyCount++;
                    // 检查是否包含张三
                    if (line.Contains("张三"))
                    {
                        Console.WriteLine("✅ 历史回放中找到了 '张三'");
                    }
                }
                
                // session/load 完成后会返回 result
                if (line.Contains(@"""id"":1") && line.Contains("result"))
                {
                    Console.WriteLine("session/load 完成，回放了 " + historyCount + " 条历史");
                    break;
                }
            }
            
            Console.WriteLine("\n=== Step 4: 发送新 prompt ===");
            Console.WriteLine("发送 prompt: '你还记得我的名字和我们讨论的话题吗？'");
            sw2.WriteLine(@"{""jsonrpc"":""2.0"",""id"":2,""method"":""session/prompt"",""params"":{""sessionId"":""" + acpSessionId + @""",""prompt"":[{""type"":""text"",""text"":""你还记得我的名字和我们讨论的话题吗？""}]}}");
            sw2.Flush();
            
            // 读取响应
            var sb = new StringBuilder();
            for (int i = 0; i < 40; i++)
            {
                var line = sr2.ReadLine();
                if (line == null) break;
                if (verbose) Console.WriteLine("Response: " + line);
                
                if (line.Contains("agent_message_chunk"))
                {
                    var textMatch = Regex.Match(line, @"""text"":""([^""]+)""");
                    if (textMatch.Success)
                    {
                        sb.Append(textMatch.Groups[1].Value);
                    }
                }
                
                if (line.Contains(@"""id"":2") && line.Contains("result")) break;
            }
            
            var response = sb.ToString();
            Console.WriteLine("\n响应: " + response);
            
            // 验证
            Console.WriteLine("\n=== Step 5: 验证结果 ===");
            bool foundName = response.Contains("张三");
            bool foundTopic = response.Contains("session/load") || response.Contains("session") || response.Contains("功能");
            
            if (foundName)
            {
                Console.WriteLine("✅ Agent 记住了名字 '张三'");
            }
            else
            {
                Console.WriteLine("⚠️ Agent 未提及名字");
            }
            
            if (foundTopic)
            {
                Console.WriteLine("✅ Agent 记住了讨论话题");
            }
            else
            {
                Console.WriteLine("⚠️ Agent 未提及话题");
            }
            
            if (foundName && foundTopic)
            {
                Console.WriteLine("\n🎉 SUCCESS: session/load 功能验证成功！上下文完全恢复！");
            }
            else if (foundName || foundTopic)
            {
                Console.WriteLine("\n⚠️ PARTIAL: session/load 部分成功，部分上下文恢复");
            }
            else
            {
                Console.WriteLine("\n❌ FAILED: session/load 未能恢复上下文");
            }
            
            sw2.Close();
            proc2.WaitForExit(3000);
        }
    }
}
'@

Add-Type -TypeDefinition $code -Language CSharp
[AcpLoadValidator]::Main(@($TestDir))

Write-Host ""
Write-Host "=== 测试完成 ===" -ForegroundColor Cyan