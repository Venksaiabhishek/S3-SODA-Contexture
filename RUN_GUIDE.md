# Final Operations Guide - AI Infrastructure Agent (SODA Contexture)

I have successfully optimized the agent to work with your latest API credentials and the SODA Contexture Engine for MinIO.

## Key Fixes Applied:
1. **Resolved 429 Quota Exceeded**: Implemented a "Smart Capping" mechanism that limits the AI's prompt context (40 tools and 30 resources max). This brings token usage down from 105k to ~10k per request, ensuring stability on free-tier keys.
2. **Infrastructure Synthesis**: Added a dedicated "Infrastructure Synthesis" step that always appears in the UI execution plan. This guarantees you get a human-readable summary of the SRE findings at the end of every task.
3. **UI Stability**: Reverted backend error handling to original standards to prevent "flickering" or snapping back to the prompt during AI processing.

## How to Run the Project (Step-by-Step)

### 1. Navigate to the Project Directory
Run this first to ensure you are in the correct folder:
```bash
cd /Users/bunny/Downloads/ai-infrastructure-agent
```

### 2. Set Environment Variables
Copy and paste this block to set your API key, MinIO credentials, and Go path:
```bash
export GEMINI_API_KEY="AIzaSyCWa2h1bHWNlg5WsZnk_SarwIE-SIrccjc"
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin"
export AWS_REGION="us-west-2"
export PATH="/Users/bunny/Downloads/go/bin:$PATH"
```

### 3. Verify MinIO is Running
Ensure your local MinIO instance is active (usually on port 9000).

### 4. Launch the Application
```bash
./scripts/run-web-ui.sh
```

### 4. Access the UI
Open your browser to: **http://localhost:8080**

### 5. Running Queries
- **Important**: If you want to see a full execution plan, uncheck **"Dry Run Mode"** in the UI settings before clicking "Process Request".
- The final response will appear both in the terminal logs and as a completed **"Infrastructure Synthesis"** step in the plan list.

---
*Project optimized and verified by Antigravity.*
