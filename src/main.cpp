#include <iostream>
#include <string>
#include <cstdlib>
#include <sstream>
const std::string BGIT_VERSION = "v1.0.0";
int main(){
std::string input;
std::string commit_msg = "update";
std::string command = "";
std::string custom_commit_message;
std::cout << "BGIT_VERSION " << BGIT_VERSION << "\n";
std::cout << "Welcome To BGIT\n";
std::cout << "Option's\n";
std::cout << "[1] Add,Commit,Push\n";
std::cout << "[2] Add,Custom Commit Message,Push\n";
std::cout << "Enter Your Choice --> ";
std::getline(std::cin,input);
std::cout << input << "\n";
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
    std::cerr << "add a number lol\n";
}
}