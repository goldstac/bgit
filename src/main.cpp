#include <iostream>
#include <string>
#include <cstdlib>
#include <sstream>
const std::string BGIT_VERSION = "v1.0.1";
int main(){
std::string input;
std::string commit_msg = "update";
std::string command = "";
std::string custom_commit_message;
std::string custom_branch_push;
std::cout << "BGIT_VERSION " << BGIT_VERSION << "\n";
std::cout << "Welcome To BGIT\n";
std::cout << "Option's\n";
std::cout << "[1] Add,Commit,Push\n";
std::cout << "[2] Add,Custom Commit Message,Push\n";
std::cout << "[3] Add,Commit,Push To Specified Branch\n";
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
else if (input == "3"){
    std::cout << "Enter Branch Name\n -->";
    std::getline(std::cin,custom_branch_push);
ss3 << "git add . && git commit -m \"update\" && git push origin "
    << custom_branch_push;
    command = ss3.str();
    std::system(command.c_str());
}
else{
    std::cerr << "add a number lol\n";
}
}