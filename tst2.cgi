#!php
<?php
echo "Content-Type: text/html\n";
echo "\n\n";

echo "<pre>";

echo $_SERVER["CGI_HEADER_ACCEPT"] . "<br />";

print_r($_SERVER);