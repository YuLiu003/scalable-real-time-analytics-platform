#!/bin/bash
set -e

echo "🔧 Fixing dependencies in all components..."

# Function to update requirements and rebuild image
fix_dependencies() {
    local component=$1
    
    echo "- Updating $component requirements..."
    
    # Create proper requirements file with compatible dependencies
    cat > $component/requirements.txt << REQUIREMENTS
flask==2.0.1
werkzeug==2.0.2
requests==2.26.0
REQUIREMENTS
    
    # Add component-specific dependencies
    case "$component" in
        "data-ingestion")
            echo "kafka-python==2.0.2" >> $component/requirements.txt
            ;;
        "processing-engine")
            echo "kafka-python==2.0.2" >> $component/requirements.txt
            echo "numpy==1.21.0" >> $component/requirements.txt
            ;;
        "storage-layer")
            echo "sqlalchemy==1.4.23" >> $component/requirements.txt
            ;;
        "visualization")
            echo "flask-socketio==5.1.1" >> $component/requirements.txt
            echo "eventlet==0.33.0" >> $component/requirements.txt
            ;;
    esac
    
    # Build the image
    echo "- Building $component image..."
    docker build -t real-time-analytics-platform-$component:latest ./$component
}

# Fix all components
components=("data-ingestion" "processing-engine" "storage-layer" "visualization")
for component in "${components[@]}"; do
    fix_dependencies "$component"
done

echo "✅ All dependencies updated and images built!"