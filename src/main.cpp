#include <iostream>
#include <string>
#include <cstdlib>
#include <sstream>
int main(){
std::string input;
std::string commit_msg = "update";
std::string command = "";
std::string custom_commit_message;
std::cout << "Welcome To BGIT\n";
std::cout << "Option's\n";
std::cout << "[1] Add,Commit,Push\n";
std::cout << "[2] Add,Custom Commit Message,Push\n";
std::cout << "Enter Your Choice --> ";
std::getline(std::cin,input);
std::cout << input;
std::stringstream ss1;
std::stringstream ss2;
std::stringstream ss3;
std::stringstream ss4;
if (input == "1"){
    ss1 << "git add . && git commit -m \"" << commit_msg << "\" && git push";
    command = ss1.str();
    std::system(command.c_str());
}
else if (input == "2"){
std::cout << "Enter Commit Message\n -->";
std::getline(std::cin,custom_commit_message);
ss2 << "git add . && git commit -m \"" << custom_commit_message << "\" && git push";
command = ss2.str();
std::system(command.c_str());
}
else{
    std::cout << "add a number lol\n";
}
}