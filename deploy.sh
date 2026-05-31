#/bin/sh
# Deploy script - configure hosts below before running
hostlist=''
# Example: hostlist='m.nrlptt.com ham.73ham.com'

time=`date "+%Y%m%d%H%M%S"`

if [ -z "$hostlist" ]; then
    echo "hostlist is empty, please configure in deploy.sh"
    exit 1
fi

for i in $hostlist ; do
echo "deploying to $i"
   scp udphub root@$i:
   ssh root@$i "cd /nrllink; mv udphub udphub.$time ; cp /root/udphub . ; systemctl restart nrllink"
done

