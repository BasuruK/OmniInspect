# This script restarts the Oracle listener inside the Docker container
# !IMPORTANT: only use if the oracle database is running inside a docker container

# Fix listener configuration and start it
docker exec oracle26ai bash -c 'sed -i "s/HOST = [^)]*)/HOST = 0.0.0.0)/g" /opt/oracle/product/26ai/dbhomeFree/network/admin/listener.ora && lsnrctl start'

# Register database services with the listener
docker exec -u oracle oracle26ai bash -c 'echo "ALTER SYSTEM REGISTER;" | sqlplus -s / as sysdba'