#!/usr/bin/env python3
import os
import time
import logging

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

def main():
    logger.info("Starting simple test app")
    count = 0
    while True:
        logger.info(f"Hello, Kubernetes! Count: {count}")
        count += 1
        time.sleep(5)

if __name__ == "__main__":
    main()
